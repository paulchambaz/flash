package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn       *sql.DB
	timeFactor float64
}

type CardState struct {
	ID         int64
	Deck       string
	Concept    string
	Reference  string
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	LastReview *time.Time
	DueDate    *time.Time
}

type Review struct {
	CardID          int64
	ReviewedAt      time.Time
	KeywordsScore   float64
	Accuracy        float64
	Correct         bool
	StabilityBefore float64
	StabilityAfter  float64
	IntervalDays    float64
}

func openDB(path string, timeFactor float64) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("wal mode: %w", err)
	}
	db := &DB{conn: conn, timeFactor: timeFactor}
	return db, db.migrate()
}

func (db *DB) close() error { return db.conn.Close() }

func (db *DB) migrate() error {
	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS cards (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			deck        TEXT    NOT NULL,
			concept     TEXT    NOT NULL,
			reference   TEXT    NOT NULL,
			stability   REAL    NOT NULL DEFAULT 0,
			difficulty  REAL    NOT NULL DEFAULT 5,
			reps        INTEGER NOT NULL DEFAULT 0,
			lapses      INTEGER NOT NULL DEFAULT 0,
			last_review DATETIME,
			due_date    DATETIME,
			UNIQUE(deck, concept)
		);
		CREATE TABLE IF NOT EXISTS reviews (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			card_id          INTEGER NOT NULL REFERENCES cards(id),
			reviewed_at      DATETIME NOT NULL,
			keywords_score   REAL NOT NULL,
			accuracy         REAL NOT NULL,
			correct          BOOLEAN NOT NULL,
			stability_before REAL NOT NULL,
			stability_after  REAL NOT NULL,
			interval_days    REAL NOT NULL
		);
	`); err != nil {
		return err
	}
	// Idempotent: add ref_hash column if not present (existing DBs).
	_, err := db.conn.Exec(`ALTER TABLE cards ADD COLUMN ref_hash TEXT NOT NULL DEFAULT ''`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name: ref_hash") {
		return err
	}
	return nil
}

func refHash(reference string) string {
	sum := sha256.Sum256([]byte(reference))
	return fmt.Sprintf("%x", sum)[:16]
}

func (db *DB) syncCards(deck string, cards []Card) error {
	incoming := make(map[string]Card, len(cards))
	for _, c := range cards {
		incoming[c.Concept] = c
	}

	rows, err := db.conn.Query(`SELECT id, concept, ref_hash FROM cards WHERE deck = ?`, deck)
	if err != nil {
		return err
	}
	type existing struct {
		id   int64
		hash string
	}
	inDB := make(map[string]existing)
	for rows.Next() {
		var e existing
		var concept string
		if err := rows.Scan(&e.id, &concept, &e.hash); err != nil {
			rows.Close()
			return err
		}
		inDB[concept] = e
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for concept, e := range inDB {
		if _, ok := incoming[concept]; !ok {
			if _, err := db.conn.Exec(`DELETE FROM reviews WHERE card_id = ?`, e.id); err != nil {
				return err
			}
			if _, err := db.conn.Exec(`DELETE FROM cards WHERE id = ?`, e.id); err != nil {
				return err
			}
		}
	}

	for _, c := range cards {
		h := refHash(c.Reference)
		if e, ok := inDB[c.Concept]; !ok {
			if _, err := db.conn.Exec(
				`INSERT INTO cards (deck, concept, reference, ref_hash) VALUES (?, ?, ?, ?)`,
				deck, c.Concept, c.Reference, h,
			); err != nil {
				return err
			}
		} else if e.hash != h {
			if _, err := db.conn.Exec(
				`UPDATE cards SET reference = ?, ref_hash = ? WHERE deck = ? AND concept = ?`,
				c.Reference, h, deck, c.Concept,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *DB) dueCards(deck string) ([]CardState, error) {
	rows, err := db.conn.Query(`
		SELECT id, deck, concept, reference, stability, difficulty, reps, lapses, last_review, due_date
		FROM cards
		WHERE deck = ? AND (due_date IS NULL OR due_date <= ?)
		ORDER BY CASE WHEN due_date IS NULL THEN 0 ELSE 1 END, due_date ASC
	`, deck, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CardState
	for rows.Next() {
		var c CardState
		var lastReview, dueDate sql.NullString
		if err := rows.Scan(&c.ID, &c.Deck, &c.Concept, &c.Reference,
			&c.Stability, &c.Difficulty, &c.Reps, &c.Lapses, &lastReview, &dueDate); err != nil {
			return nil, err
		}
		var terr error
		c.LastReview, terr = scanNullTime(lastReview)
		if terr != nil {
			return nil, terr
		}
		c.DueDate, terr = scanNullTime(dueDate)
		if terr != nil {
			return nil, terr
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (db *DB) updateCard(c CardState) error {
	_, err := db.conn.Exec(`
		UPDATE cards
		SET stability=?, difficulty=?, reps=?, lapses=?, last_review=?, due_date=?
		WHERE id=?
	`, c.Stability, c.Difficulty, c.Reps, c.Lapses, c.LastReview, c.DueDate, c.ID)
	return err
}

func (db *DB) resetDeck(deck string) error {
	_, err := db.conn.Exec(`
		DELETE FROM reviews WHERE card_id IN (SELECT id FROM cards WHERE deck = ?)
	`, deck)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec(`
		UPDATE cards
		SET stability=0, difficulty=5, reps=0, lapses=0, last_review=NULL, due_date=NULL
		WHERE deck = ?
	`, deck)
	return err
}

func (db *DB) deckTotal(deck string) (int, error) {
	var n int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM cards WHERE deck = ?`, deck).Scan(&n)
	return n, err
}

func (db *DB) dueCount(deck string) (int, error) {
	var n int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM cards
		WHERE deck = ? AND (due_date IS NULL OR due_date <= ?)
	`, deck, time.Now().UTC()).Scan(&n)
	return n, err
}

func scanNullTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	for _, f := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(f, s.String); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("cannot parse time: %q", s.String)
}

func (db *DB) lastReviewTime(deck string) (*time.Time, error) {
	var s sql.NullString
	err := db.conn.QueryRow(`
		SELECT MAX(r.reviewed_at) FROM reviews r
		JOIN cards c ON c.id = r.card_id
		WHERE c.deck = ?
	`, deck).Scan(&s)
	if err != nil {
		return nil, err
	}
	return scanNullTime(s)
}

func (db *DB) getCard(id int64) (CardState, error) {
	var c CardState
	var lastReview, dueDate sql.NullString
	err := db.conn.QueryRow(`
		SELECT id, deck, concept, reference, stability, difficulty, reps, lapses, last_review, due_date
		FROM cards WHERE id = ?
	`, id).Scan(&c.ID, &c.Deck, &c.Concept, &c.Reference,
		&c.Stability, &c.Difficulty, &c.Reps, &c.Lapses, &lastReview, &dueDate)
	if err != nil {
		return CardState{}, err
	}
	c.LastReview, err = scanNullTime(lastReview)
	if err != nil {
		return CardState{}, err
	}
	c.DueDate, err = scanNullTime(dueDate)
	if err != nil {
		return CardState{}, err
	}
	return c, nil
}

func (db *DB) submitReview(cardID int64, correct bool, accuracy, keywordsScore float64) (schedResult, error) {
	return db.submitReviewWithFactor(cardID, correct, accuracy, keywordsScore, db.timeFactor)
}

func (db *DB) submitReviewWithFactor(cardID int64, correct bool, accuracy, keywordsScore, timeFactor float64) (schedResult, error) {
	card, err := db.getCard(cardID)
	if err != nil {
		return schedResult{}, fmt.Errorf("get card: %w", err)
	}

	stabilityBefore := card.Stability
	now := time.Now()
	sr := schedule(card, correct, accuracy, timeFactor, now)

	nowUTC := now.UTC()
	nextDue := sr.nextDue.UTC()
	card.Stability = sr.stability
	card.Difficulty = sr.difficulty
	card.Reps++
	if !correct {
		card.Lapses++
	}
	card.LastReview = &nowUTC
	card.DueDate = &nextDue

	if err := db.updateCard(card); err != nil {
		return schedResult{}, fmt.Errorf("update card: %w", err)
	}
	if err := db.addReview(Review{
		CardID:          cardID,
		ReviewedAt:      nowUTC,
		KeywordsScore:   keywordsScore,
		Accuracy:        accuracy,
		Correct:         correct,
		StabilityBefore: stabilityBefore,
		StabilityAfter:  sr.stability,
		IntervalDays:    sr.intervalDays,
	}); err != nil {
		return schedResult{}, fmt.Errorf("add review: %w", err)
	}

	return sr, nil
}

type DeckStats struct {
	Total        int
	New          int
	Due          int
	SuccessRate  float64
	ReviewCount  int
	AvgStability float64
	LastReview   *time.Time
}

func (db *DB) deckStats(deck string) (DeckStats, error) {
	var s DeckStats

	row := db.conn.QueryRow(`
		SELECT
			COUNT(*),
			SUM(CASE WHEN reps = 0 THEN 1 ELSE 0 END),
			SUM(CASE WHEN due_date IS NULL OR due_date <= ? THEN 1 ELSE 0 END),
			AVG(CASE WHEN reps > 0 THEN stability ELSE NULL END)
		FROM cards WHERE deck = ?
	`, time.Now().UTC(), deck)
	var avgStab sql.NullFloat64
	if err := row.Scan(&s.Total, &s.New, &s.Due, &avgStab); err != nil {
		return DeckStats{}, err
	}
	if avgStab.Valid {
		s.AvgStability = avgStab.Float64
	}

	row = db.conn.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN correct THEN 1 ELSE 0 END)
		FROM (
			SELECT r.correct FROM reviews r
			JOIN cards c ON c.id = r.card_id
			WHERE c.deck = ?
			ORDER BY r.reviewed_at DESC
			LIMIT 30
		)
	`, deck)
	var total, correct sql.NullInt64
	if err := row.Scan(&total, &correct); err != nil {
		return DeckStats{}, err
	}
	if total.Valid && total.Int64 > 0 {
		s.ReviewCount = int(total.Int64)
		s.SuccessRate = float64(correct.Int64) / float64(total.Int64)
	}

	var ts sql.NullString
	err := db.conn.QueryRow(`
		SELECT MAX(r.reviewed_at) FROM reviews r
		JOIN cards c ON c.id = r.card_id
		WHERE c.deck = ?
	`, deck).Scan(&ts)
	if err != nil {
		return DeckStats{}, err
	}
	s.LastReview, err = scanNullTime(ts)
	if err != nil {
		return DeckStats{}, err
	}

	return s, nil
}

func (db *DB) addReview(r Review) error {
	_, err := db.conn.Exec(`
		INSERT INTO reviews
			(card_id, reviewed_at, keywords_score, accuracy, correct,
			 stability_before, stability_after, interval_days)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, r.CardID, r.ReviewedAt, r.KeywordsScore, r.Accuracy, r.Correct,
		r.StabilityBefore, r.StabilityAfter, r.IntervalDays)
	return err
}
