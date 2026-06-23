package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestFlashServer(t *testing.T) (*httptest.Server, *flashServer) {
	t.Helper()
	dir := t.TempDir()
	s := &flashServer{
		token:   "secret",
		dataDir: dir,
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
	mux.HandleFunc("POST /decks/{deck}/reset", s.auth(s.handleReset))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s
}

func authRequest(t *testing.T, method, url string, body []byte, contentType string) *http.Request {
	t.Helper()
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req
}

func do(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestServeAuthMiddleware(t *testing.T) {
	srv, _ := newTestFlashServer(t)

	t.Run("missing token → 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", srv.URL+"/decks", nil)
		resp := do(t, req)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("wrong token → 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", srv.URL+"/decks", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		resp := do(t, req)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("correct token → 200", func(t *testing.T) {
		req := authRequest(t, "GET", srv.URL+"/decks", nil, "")
		resp := do(t, req)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})
}

func pushDeckToServer(t *testing.T, srv *httptest.Server, deckName, content string) {
	t.Helper()
	url := fmt.Sprintf("%s/decks/%s/push", srv.URL, deckName)
	req := authRequest(t, "POST", url, []byte(content), "text/plain")
	resp := do(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestServeHandlePush(t *testing.T) {
	srv, _ := newTestFlashServer(t)
	content := "Question A\nAnswer A\n\nQuestion B\nAnswer B\n"
	pushDeckToServer(t, srv, "testdeck", content)

	// Verify card count via /total
	req := authRequest(t, "GET", srv.URL+"/decks/testdeck/total", nil, "")
	resp := do(t, req)
	defer resp.Body.Close()

	var res struct {
		Total int `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if res.Total != 2 {
		t.Errorf("total = %d, want 2", res.Total)
	}
}

func TestServeHandleDueCards(t *testing.T) {
	srv, _ := newTestFlashServer(t)
	pushDeckToServer(t, srv, "testdeck", "Q\nA\n")

	req := authRequest(t, "GET", srv.URL+"/decks/testdeck/cards/due", nil, "")
	resp := do(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var cards []CardState
	json.NewDecoder(resp.Body).Decode(&cards)
	if len(cards) != 1 {
		t.Errorf("len(cards) = %d, want 1", len(cards))
	}
}

func TestServeHandleReview(t *testing.T) {
	srv, _ := newTestFlashServer(t)
	pushDeckToServer(t, srv, "testdeck", "Q\nA\n")

	dueReq := authRequest(t, "GET", srv.URL+"/decks/testdeck/cards/due", nil, "")
	dueResp := do(t, dueReq)
	var cards []CardState
	json.NewDecoder(dueResp.Body).Decode(&cards)
	dueResp.Body.Close()

	body, _ := json.Marshal(reviewRequest{
		CardID:        cards[0].ID,
		Correct:       true,
		Accuracy:      0.9,
		KeywordsScore: 1.0,
		PaceSeconds:   86400.0,
	})
	req := authRequest(t, "POST", srv.URL+"/decks/testdeck/cards/review", body, "application/json")
	resp := do(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var rr reviewResponse
	json.NewDecoder(resp.Body).Decode(&rr)
	if rr.IntervalDays == 0 {
		t.Error("expected non-zero intervalDays")
	}
}

func TestServeHandleReset(t *testing.T) {
	srv, _ := newTestFlashServer(t)
	pushDeckToServer(t, srv, "testdeck", "Q\nA\n")

	// Submit a review first
	dueReq := authRequest(t, "GET", srv.URL+"/decks/testdeck/cards/due", nil, "")
	dueResp := do(t, dueReq)
	var cards []CardState
	json.NewDecoder(dueResp.Body).Decode(&cards)
	dueResp.Body.Close()

	body, _ := json.Marshal(reviewRequest{CardID: cards[0].ID, Correct: true, Accuracy: 1.0, KeywordsScore: 1.0, PaceSeconds: 86400.0})
	reviewReq := authRequest(t, "POST", srv.URL+"/decks/testdeck/cards/review", body, "application/json")
	do(t, reviewReq).Body.Close()

	// Reset
	req := authRequest(t, "POST", srv.URL+"/decks/testdeck/reset", nil, "")
	resp := do(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reset status = %d, want 200", resp.StatusCode)
	}
}

func TestServeHandleShowDeck(t *testing.T) {
	srv, _ := newTestFlashServer(t)
	pushDeckToServer(t, srv, "testdeck", "Q\nA\n")

	req := authRequest(t, "GET", srv.URL+"/decks/testdeck", nil, "")
	resp := do(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var res deckShowResponse
	json.NewDecoder(resp.Body).Decode(&res)
	if res.Total != 1 {
		t.Errorf("Total = %d, want 1", res.Total)
	}
}

func TestServeHandleDeckStats(t *testing.T) {
	srv, _ := newTestFlashServer(t)
	pushDeckToServer(t, srv, "testdeck", "Q\nA\n\nQ2\nA2\n")

	req := authRequest(t, "GET", srv.URL+"/decks/testdeck/stats", nil, "")
	resp := do(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var stats DeckStats
	json.NewDecoder(resp.Body).Decode(&stats)
	if stats.Total != 2 {
		t.Errorf("Total = %d, want 2", stats.Total)
	}
}

func TestServeHandlePullDeck(t *testing.T) {
	srv, s := newTestFlashServer(t)
	content := "Q\nA\n"
	mdPath := filepath.Join(s.dataDir, "testdeck.md")
	os.WriteFile(mdPath, []byte(content), 0o644)

	req := authRequest(t, "GET", srv.URL+"/decks/testdeck/content", nil, "")
	resp := do(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := make([]byte, len(content))
	resp.Body.Read(body)
	if string(body) != content {
		t.Errorf("content = %q, want %q", body, content)
	}
}

func TestServeHandlePullDeckNotFound(t *testing.T) {
	srv, _ := newTestFlashServer(t)

	req := authRequest(t, "GET", srv.URL+"/decks/nonexistent/content", nil, "")
	resp := do(t, req)
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestServeHandleDeleteDeck(t *testing.T) {
	srv, s := newTestFlashServer(t)
	pushDeckToServer(t, srv, "testdeck", "Q\nA\n")

	req := authRequest(t, "DELETE", srv.URL+"/decks/testdeck", nil, "")
	resp := do(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Files should be gone
	for _, ext := range []string{".db", ".md"} {
		path := filepath.Join(s.dataDir, "testdeck"+ext)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("file %s still exists after delete", path)
		}
	}
}

func TestServeHandleListDecks(t *testing.T) {
	srv, _ := newTestFlashServer(t)
	pushDeckToServer(t, srv, "deck1", "Q\nA\n")
	pushDeckToServer(t, srv, "deck2", "Q\nA\n")

	req := authRequest(t, "GET", srv.URL+"/decks", nil, "")
	resp := do(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var items []deckListItem
	json.NewDecoder(resp.Body).Decode(&items)
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}
