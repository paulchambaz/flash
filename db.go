package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ conn *sql.DB }

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

func openDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("wal mode: %w", err)
	}
	db := &DB{conn: conn}
	return db, db.migrate()
}

func (db *DB) close() error { return db.conn.Close() }

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
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
	`)
	return err
}

func (db *DB) syncCards(deck string, cards []Card) error {
	for _, c := range cards {
		if _, err := db.conn.Exec(`
			INSERT OR IGNORE INTO cards (deck, concept, reference) VALUES (?, ?, ?)
		`, deck, c.Concept, c.Reference); err != nil {
			return err
		}
		if _, err := db.conn.Exec(`
			UPDATE cards SET reference = ? WHERE deck = ? AND concept = ?
		`, c.Reference, deck, c.Concept); err != nil {
			return err
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
		var lastReview, dueDate sql.NullTime
		if err := rows.Scan(&c.ID, &c.Deck, &c.Concept, &c.Reference,
			&c.Stability, &c.Difficulty, &c.Reps, &c.Lapses, &lastReview, &dueDate); err != nil {
			return nil, err
		}
		if lastReview.Valid {
			c.LastReview = &lastReview.Time
		}
		if dueDate.Valid {
			c.DueDate = &dueDate.Time
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

func (db *DB) lastReviewTime(deck string) (*time.Time, error) {
	var t sql.NullTime
	err := db.conn.QueryRow(`
		SELECT MAX(r.reviewed_at) FROM reviews r
		JOIN cards c ON c.id = r.card_id
		WHERE c.deck = ?
	`, deck).Scan(&t)
	if err != nil || !t.Valid {
		return nil, err
	}
	return &t.Time, nil
}

func (db *DB) getCard(id int64) (CardState, error) {
	var c CardState
	var lastReview, dueDate sql.NullTime
	err := db.conn.QueryRow(`
		SELECT id, deck, concept, reference, stability, difficulty, reps, lapses, last_review, due_date
		FROM cards WHERE id = ?
	`, id).Scan(&c.ID, &c.Deck, &c.Concept, &c.Reference,
		&c.Stability, &c.Difficulty, &c.Reps, &c.Lapses, &lastReview, &dueDate)
	if err != nil {
		return CardState{}, err
	}
	if lastReview.Valid {
		c.LastReview = &lastReview.Time
	}
	if dueDate.Valid {
		c.DueDate = &dueDate.Time
	}
	return c, nil
}

func (db *DB) submitReview(cardID int64, correct bool, accuracy, keywordsScore float64) (schedResult, error) {
	card, err := db.getCard(cardID)
	if err != nil {
		return schedResult{}, fmt.Errorf("get card: %w", err)
	}

	stabilityBefore := card.Stability
	now := time.Now()
	sr := schedule(card, correct, accuracy, now)

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
