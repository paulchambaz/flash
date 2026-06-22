package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testToken = "test-bearer-token"

func newTestRemoteStore(t *testing.T, srv *httptest.Server) *remoteStore {
	t.Helper()
	return &remoteStore{
		baseURL: srv.URL,
		deck:    "testdeck",
		token:   testToken,
		step:    24 * time.Hour,
		client:  srv.Client(),
	}
}

func TestRemoteStoreBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode([]CardState{})
	}))
	defer srv.Close()

	rs := newTestRemoteStore(t, srv)
	rs.dueCards("testdeck")

	if gotAuth != "Bearer "+testToken {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer "+testToken)
	}
}

func TestRemoteStoreDueCards(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]CardState{
			{ID: 1, Deck: "testdeck", Concept: "Q1", Reference: "R1"},
			{ID: 2, Deck: "testdeck", Concept: "Q2", Reference: "R2", DueDate: &now},
		})
	}))
	defer srv.Close()

	rs := newTestRemoteStore(t, srv)
	cards, err := rs.dueCards("testdeck")
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("len(cards) = %d, want 2", len(cards))
	}
	if cards[0].Concept != "Q1" {
		t.Errorf("cards[0].Concept = %q", cards[0].Concept)
	}
}

func TestRemoteStoreDeckTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]int{"total": 42})
	}))
	defer srv.Close()

	rs := newTestRemoteStore(t, srv)
	n, err := rs.deckTotal("testdeck")
	if err != nil {
		t.Fatal(err)
	}
	if n != 42 {
		t.Errorf("deckTotal = %d, want 42", n)
	}
}

func TestRemoteStoreSubmitReview(t *testing.T) {
	nextDue := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)
	var gotBody reviewRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(reviewResponse{
			Stability:    3.5,
			Difficulty:   5.0,
			IntervalDays: 3.0,
			NextDue:      nextDue,
		})
	}))
	defer srv.Close()

	rs := newTestRemoteStore(t, srv)
	sr, err := rs.submitReview(7, true, 0.9, 0.85)
	if err != nil {
		t.Fatal(err)
	}

	if gotBody.CardID != 7 {
		t.Errorf("CardID = %d, want 7", gotBody.CardID)
	}
	if !gotBody.Correct {
		t.Error("expected Correct=true in request")
	}
	if sr.intervalDays != 3.0 {
		t.Errorf("intervalDays = %v, want 3.0", sr.intervalDays)
	}
}

func TestRemoteStorePushDeck(t *testing.T) {
	var gotBody []byte
	var gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]int{"synced": 1})
	}))
	defer srv.Close()

	content := []byte("Concept\nReference\n")
	rs := newTestRemoteStore(t, srv)
	if err := rs.pushDeck(content); err != nil {
		t.Fatal(err)
	}
	if string(gotBody) != string(content) {
		t.Errorf("body = %q, want %q", gotBody, content)
	}
	if gotContentType != "text/plain" {
		t.Errorf("Content-Type = %q, want text/plain", gotContentType)
	}
}

func TestRemoteStorePullDeck(t *testing.T) {
	content := []byte("Concept\nReference\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(content)
	}))
	defer srv.Close()

	rs := newTestRemoteStore(t, srv)
	got, err := rs.pullDeck()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("pullDeck = %q, want %q", got, content)
	}
}

func TestRemoteStoreError401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	}))
	defer srv.Close()

	rs := newTestRemoteStore(t, srv)
	_, err := rs.dueCards("testdeck")
	if err == nil {
		t.Error("expected error on 401")
	}
}

func TestRemoteStoreError500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	}))
	defer srv.Close()

	rs := newTestRemoteStore(t, srv)
	_, err := rs.deckTotal("testdeck")
	if err == nil {
		t.Error("expected error on 500")
	}
}

func TestRemoteStoreListDecks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]deckListItem{
			{Name: "deck1", Total: 10, Due: 3},
			{Name: "deck2", Total: 5, Due: 0},
		})
	}))
	defer srv.Close()

	rs := newTestRemoteStore(t, srv)
	items, err := rs.listDecks()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Name != "deck1" {
		t.Errorf("items[0].Name = %q", items[0].Name)
	}
}

func TestRemoteStoreResetDeck(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		json.NewEncoder(w).Encode(map[string]bool{"reset": true})
	}))
	defer srv.Close()

	rs := newTestRemoteStore(t, srv)
	if err := rs.resetDeck(); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
}
