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
	baseURL string
	deck    string
	token   string
	client  *http.Client
}

func newRemoteStore(host, deck, token string, port int) *remoteStore {
	return &remoteStore{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		deck:    deck,
		token:   token,
		client:  &http.Client{Timeout: 15 * time.Second},
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
