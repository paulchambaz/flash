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
	mu      sync.RWMutex
	dbs     map[string]*DB
}

type reviewRequest struct {
	CardID        int64   `json:"card_id"`
	Correct       bool    `json:"correct"`
	Accuracy      float64 `json:"accuracy"`
	KeywordsScore float64 `json:"keywords_score"`
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
		dbs:     make(map[string]*DB),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /decks/{deck}/push",         s.auth(s.handlePush))
	mux.HandleFunc("GET /decks/{deck}/cards/due",     s.auth(s.handleDueCards))
	mux.HandleFunc("GET /decks/{deck}/total",         s.auth(s.handleDeckTotal))
	mux.HandleFunc("POST /decks/{deck}/cards/review", s.auth(s.handleReview))
	mux.HandleFunc("POST /decks/{deck}/reset",        s.auth(s.handleReset))

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
	db, err := openDB(dbPath)
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
	sr, err := db.submitReview(req.CardID, req.Correct, req.Accuracy, req.KeywordsScore)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// deckNameFromPath strips directory and extension: "path/to/deck.md" → "deck"
func deckNameFromPath(s string) string {
	return strings.TrimSuffix(filepath.Base(s), filepath.Ext(s))
}
