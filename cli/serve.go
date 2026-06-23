package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type flashServer struct {
	token   string
	dataDir string
	evalCfg evalConfig
	pace    time.Duration
	mu      sync.RWMutex
	dbs     map[string]*DB
}

type reviewRequest struct {
	CardID        int64   `json:"card_id"`
	Correct       bool    `json:"correct"`
	Accuracy      float64 `json:"accuracy"`
	KeywordsScore float64 `json:"keywords_score"`
	PaceSeconds   float64 `json:"pace_seconds"`
}

type reviewResponse struct {
	Stability       float64   `json:"stability"`
	Difficulty      float64   `json:"difficulty"`
	IntervalDays    float64   `json:"interval_days"`
	NextDue         time.Time `json:"next_due"`
	ReshowInSession bool      `json:"reshow_in_session"`
}

func runServe(cfg appConfig) error {
	if cfg.ServeToken == "" {
		return fmt.Errorf("serve_token must be set in flash.cfg or FLASH_SERVE_TOKEN")
	}
	dataDir := cfg.ServeData
	if dataDir == "" {
		dataDir = "."
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	s := &flashServer{
		token:   cfg.ServeToken,
		dataDir: dataDir,
		evalCfg: evalConfigFrom(cfg),
		pace:    cfg.Pace,
		dbs:     make(map[string]*DB),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /decks", s.auth(s.handleListDecks))
	mux.HandleFunc("GET /decks/{deck}", s.auth(s.handleShowDeck))
	mux.HandleFunc("GET /decks/{deck}/stats", s.auth(s.handleDeckStats))
	mux.HandleFunc("GET /decks/{deck}/content", s.auth(s.handlePullDeck))
	mux.HandleFunc("DELETE /decks/{deck}", s.auth(s.handleDeleteDeck))
	mux.HandleFunc("POST /decks/{deck}/push", s.auth(s.handlePush))
	mux.HandleFunc("GET /decks/{deck}/cards/due", s.auth(s.handleDueCards))
	mux.HandleFunc("GET /decks/{deck}/total", s.auth(s.handleDeckTotal))
	mux.HandleFunc("POST /decks/{deck}/cards/review", s.auth(s.handleReview))
	mux.HandleFunc("POST /decks/{deck}/cards/evaluate", s.auth(s.handleEvaluate))
	mux.HandleFunc("POST /decks/{deck}/reset", s.auth(s.handleReset))
	mux.HandleFunc("GET /activity", s.auth(s.handleActivity))

	addr := fmt.Sprintf("%s:%d", cfg.ServeHost, cfg.ServePort)
	log.Printf("flash serve on %s  data=%s", addr, dataDir)
	return http.ListenAndServe(addr, mux)
}

func (s *flashServer) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+s.token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *flashServer) getDB(deck string) (*DB, error) {
	s.mu.RLock()
	db, ok := s.dbs[deck]
	s.mu.RUnlock()
	if ok {
		return db, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check after acquiring write lock
	if db, ok = s.dbs[deck]; ok {
		return db, nil
	}
	dbPath := filepath.Join(s.dataDir, deck+".db")
	db, err := openDB(dbPath, s.pace)
	if err != nil {
		return nil, err
	}
	s.dbs[deck] = db
	return db, nil
}

func (s *flashServer) handlePush(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}

	mdPath := filepath.Join(s.dataDir, deck+".md")
	if err := os.WriteFile(mdPath, body, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write deck: " + err.Error()})
		return
	}

	cards, err := parseDeck(mdPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parse deck: " + err.Error()})
		return
	}

	db, err := s.getDB(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := db.syncCards(deck, cards); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"synced": len(cards)})
}

func (s *flashServer) handleDueCards(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")
	db, err := s.getDB(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cards, err := db.dueCards(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cards)
}

func (s *flashServer) handleDeckTotal(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")
	db, err := s.getDB(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	n, err := db.deckTotal(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"total": n})
}

func (s *flashServer) handleReview(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")
	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "decode body: " + err.Error()})
		return
	}

	db, err := s.getDB(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	pace := s.pace
	if req.PaceSeconds > 0 {
		pace = time.Duration(req.PaceSeconds * float64(time.Second))
	}
	sr, err := db.submitReviewWithPace(req.CardID, req.Correct, req.Accuracy, req.KeywordsScore, pace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, reviewResponse{
		Stability:       sr.stability,
		Difficulty:      sr.difficulty,
		IntervalDays:    sr.intervalDays,
		NextDue:         sr.nextDue,
		ReshowInSession: sr.reshowInSession,
	})
}

func (s *flashServer) handleReset(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")
	db, err := s.getDB(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := db.resetDeck(deck); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"reset": true})
}

type deckShowResponse struct {
	Total      int        `json:"total"`
	Due        int        `json:"due"`
	LastReview *time.Time `json:"last_review"`
}

func (s *flashServer) handleShowDeck(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")
	db, err := s.getDB(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	total, err := db.deckTotal(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	due, err := db.dueCount(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	last, err := db.lastReviewTime(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deckShowResponse{Total: total, Due: due, LastReview: last})
}

func (s *flashServer) handleDeleteDeck(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")

	s.mu.Lock()
	if db, ok := s.dbs[deck]; ok {
		_ = db.close()
		delete(s.dbs, deck)
	}
	s.mu.Unlock()

	var errs []string
	for _, ext := range []string{".db", ".md"} {
		path := filepath.Join(s.dataDir, deck+ext)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": strings.Join(errs, "; ")})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type deckListItem struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
	Due   int    `json:"due"`
}

func (s *flashServer) handleListDecks(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var items []deckListItem
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".db" {
			name := strings.TrimSuffix(e.Name(), ".db")
			db, err := s.getDB(name)
			if err != nil {
				continue
			}
			total, _ := db.deckTotal(name)
			due, _ := db.dueCount(name)
			items = append(items, deckListItem{Name: name, Total: total, Due: due})
		}
	}
	if items == nil {
		items = []deckListItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *flashServer) handleDeckStats(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")
	db, err := s.getDB(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	stats, err := db.deckStats(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *flashServer) handlePullDeck(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")
	mdPath := filepath.Join(s.dataDir, deck+".md")
	body, err := os.ReadFile(mdPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "deck not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

type evaluateRequest struct {
	CardID      int64   `json:"card_id"`
	Answer      string  `json:"answer"`
	PaceSeconds float64 `json:"pace_seconds"`
	Threshold   float64 `json:"threshold"`
}

type evaluateResponse struct {
	Correct         bool      `json:"correct"`
	Accuracy        float64   `json:"accuracy"`
	KeywordsScore   float64   `json:"keywords_score"`
	ReshowInSession bool      `json:"reshow_in_session"`
	NextDue         time.Time `json:"next_due"`
}

func (s *flashServer) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	deck := r.PathValue("deck")
	var req evaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "decode body: " + err.Error()})
		return
	}

	db, err := s.getDB(deck)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	card, err := db.getCard(req.CardID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "card not found"})
		return
	}

	cfg := s.evalCfg
	if req.Threshold > 0 {
		cfg.threshold = req.Threshold
	}
	ev := newEvaluator(cfg)
	result, err := ev.evaluate(card.Concept, card.Reference, req.Answer)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "eval: " + err.Error()})
		return
	}

	pace := s.pace
	if req.PaceSeconds > 0 {
		pace = time.Duration(req.PaceSeconds * float64(time.Second))
	}
	sr, err := db.submitReviewWithPace(card.ID, result.correct, result.accuracy, result.keywordsScore, pace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "submit review: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, evaluateResponse{
		Correct:         result.correct,
		Accuracy:        result.accuracy,
		KeywordsScore:   result.keywordsScore,
		ReshowInSession: sr.reshowInSession,
		NextDue:         sr.nextDue,
	})
}

type DayActivity struct {
	Date string `json:"date"`
	Done int    `json:"done"`
	Due  int    `json:"due"`
}

func computeActivity(reviews []ReviewEntry, days int) []DayActivity {
	cardMap := make(map[int64][]ReviewEntry)
	for _, r := range reviews {
		cardMap[r.CardID] = append(cardMap[r.CardID], r)
	}

	now := time.Now().UTC()
	jan1 := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	weekday := int(jan1.Weekday()) // 0=Sun … 6=Sat
	if weekday == 0 {
		weekday = 7 // treat Sunday as 7 so Monday=1 is the anchor
	}
	start := jan1.AddDate(0, 0, 1-weekday) // back to Monday of that week
	out := make([]DayActivity, days)
	for i := range out {
		d := start.AddDate(0, 0, i)
		out[i].Date = d.Format("2006-01-02")
		if d.After(now) {
			continue
		}
		dEnd := d.Add(24 * time.Hour)

		done, missed := 0, 0
		for _, rr := range cardMap {
			reviewedOnD := false
			for _, r := range rr {
				rt := r.ReviewedAt.UTC()
				if !rt.Before(d) && rt.Before(dEnd) {
					reviewedOnD = true
					break
				}
			}
			if reviewedOnD {
				done++
				continue
			}
			var lastBefore *ReviewEntry
			for j := range rr {
				if rr[j].ReviewedAt.UTC().Before(d) {
					lastBefore = &rr[j]
				}
			}
			if lastBefore != nil {
				dueAt := lastBefore.ReviewedAt.UTC().Add(
					time.Duration(float64(24*time.Hour) * lastBefore.IntervalDays))
				if !dueAt.After(dEnd) {
					missed++
				}
			}
		}
		out[i].Done = done
		out[i].Due = done + missed
	}
	return out
}

func (s *flashServer) handleActivity(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var all []ReviewEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		deck := strings.TrimSuffix(e.Name(), ".db")
		db, err := s.getDB(deck)
		if err != nil {
			continue
		}
		rr, err := db.allReviews()
		if err != nil {
			continue
		}
		all = append(all, rr...)
	}
	writeJSON(w, http.StatusOK, computeActivity(all, 378))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// deckNameFromPath strips directory and extension: "path/to/deck.md" → "deck"
func deckNameFromPath(s string) string {
	return strings.TrimSuffix(filepath.Base(s), filepath.Ext(s))
}
