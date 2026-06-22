package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type remoteStore struct {
	baseURL    string
	deck       string
	token      string
	timeFactor float64
	client     *http.Client
}

func newRemoteStore(host, deck, token string, port int, timeFactor float64) *remoteStore {
	return &remoteStore{
		baseURL:    fmt.Sprintf("https://%s:%d", host, port),
		deck:       deck,
		token:      token,
		timeFactor: timeFactor,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (r *remoteStore) dueCards(_ string) ([]CardState, error) {
	resp, err := r.do("GET", "/decks/"+r.deck+"/cards/due", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var cards []CardState
	return cards, json.NewDecoder(resp.Body).Decode(&cards)
}

func (r *remoteStore) deckTotal(_ string) (int, error) {
	resp, err := r.do("GET", "/decks/"+r.deck+"/total", nil, "")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var res struct {
		Total int `json:"total"`
	}
	return res.Total, json.NewDecoder(resp.Body).Decode(&res)
}

func (r *remoteStore) submitReview(cardID int64, correct bool, accuracy, keywordsScore float64) (schedResult, error) {
	body, err := json.Marshal(reviewRequest{
		CardID:        cardID,
		Correct:       correct,
		Accuracy:      accuracy,
		KeywordsScore: keywordsScore,
		TimeFactor:    r.timeFactor,
	})
	if err != nil {
		return schedResult{}, err
	}

	resp, err := r.do("POST", "/decks/"+r.deck+"/cards/review", bytes.NewReader(body), "application/json")
	if err != nil {
		return schedResult{}, err
	}
	defer resp.Body.Close()

	var rr reviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return schedResult{}, err
	}
	return schedResult{
		stability:       rr.Stability,
		difficulty:      rr.Difficulty,
		intervalDays:    rr.IntervalDays,
		nextDue:         rr.NextDue,
		reshowInSession: rr.ReshowInSession,
	}, nil
}

func (r *remoteStore) close() error { return nil }

func (r *remoteStore) deleteDeck() error {
	resp, err := r.do("DELETE", "/decks/"+r.deck, nil, "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (r *remoteStore) showDeck() (deckShowResponse, error) {
	resp, err := r.do("GET", "/decks/"+r.deck, nil, "")
	if err != nil {
		return deckShowResponse{}, err
	}
	defer resp.Body.Close()
	var res deckShowResponse
	return res, json.NewDecoder(resp.Body).Decode(&res)
}

func (r *remoteStore) listDecks() ([]deckListItem, error) {
	resp, err := r.do("GET", "/decks", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var items []deckListItem
	return items, json.NewDecoder(resp.Body).Decode(&items)
}

func (r *remoteStore) deckStats() (DeckStats, error) {
	resp, err := r.do("GET", "/decks/"+r.deck+"/stats", nil, "")
	if err != nil {
		return DeckStats{}, err
	}
	defer resp.Body.Close()
	var s DeckStats
	return s, json.NewDecoder(resp.Body).Decode(&s)
}

func (r *remoteStore) pullDeck() ([]byte, error) {
	resp, err := r.do("GET", "/decks/"+r.deck+"/content", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// pushDeck uploads local deck content to the server.
func (r *remoteStore) pushDeck(content []byte) error {
	resp, err := r.do("POST", "/decks/"+r.deck+"/push", bytes.NewReader(content), "text/plain")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// resetDeck resets all card states on the server for this deck.
func (r *remoteStore) resetDeck() error {
	resp, err := r.do("POST", "/decks/"+r.deck+"/reset", nil, "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (r *remoteStore) do(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, r.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", method, path, err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return nil, fmt.Errorf("server %d: %s", resp.StatusCode, e.Error)
	}
	return resp, nil
}
