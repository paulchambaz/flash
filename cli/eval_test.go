package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Hello World", "hello world"},
		{"  trim  ", "trim"},
		{"café", "cafe"},
		{"naïve", "naive"},
		{"foo,bar!", "foo bar "},
		{"Système d'exploitation", "systeme d exploitation"},
	}
	for _, tt := range tests {
		got := normalizeText(tt.in)
		if got != tt.want {
			t.Errorf("normalizeText(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripBold(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"**word**", "word"},
		{"**a** and **b**", "a and b"},
		{"no bold here", "no bold here"},
		{"**multi word bold**", "multi word bold"},
	}
	for _, tt := range tests {
		got := stripBold(tt.in)
		if got != tt.want {
			t.Errorf("stripBold(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLevenshteinSim(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"", "", 1.0},
		{"abc", "abc", 1.0},
		{"abc", "abd", 1.0 - 1.0/3.0},
		{"a", "b", 0.0},
		{"kitten", "sitting", 1.0 - 3.0/7.0},
	}
	for _, tt := range tests {
		got := levenshteinSim([]rune(tt.a), []rune(tt.b))
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("levenshteinSim(%q,%q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestPartialRatio(t *testing.T) {
	tests := []struct {
		short, long string
		wantAtLeast float64
	}{
		{"abc", "abc", 1.0},
		{"abc", "xabcy", 1.0},
		{"abc", "xyz", 0.0},
		{"", "anything", 1.0},
	}
	for _, tt := range tests {
		got := partialRatio(tt.short, tt.long)
		if got < tt.wantAtLeast-1e-9 {
			t.Errorf("partialRatio(%q,%q) = %v, want >= %v", tt.short, tt.long, got, tt.wantAtLeast)
		}
	}
}

func TestPartialRatioShortLongerThanLong(t *testing.T) {
	got := partialRatio("abcdef", "abc")
	if got < 0 || got > 1 {
		t.Errorf("partialRatio out of [0,1]: %v", got)
	}
}

func TestKeywordsScoreNoBold(t *testing.T) {
	got := keywordsScore("no bold text here", "anything")
	if got != 1.0 {
		t.Errorf("no bold keywords: score = %v, want 1.0", got)
	}
}

func TestKeywordsScoreAllFound(t *testing.T) {
	ref := "La **photosynthèse** convertit la **lumière** en énergie."
	answer := "photosynthèse et lumière"
	got := keywordsScore(ref, answer)
	if got < 0.9 {
		t.Errorf("all keywords present: score = %v, want >= 0.9", got)
	}
}

func TestKeywordsScoreMissing(t *testing.T) {
	ref := "La **photosynthèse** utilise le **chlorophylle**."
	answer := "rien de pertinent ici xyz"
	got := keywordsScore(ref, answer)
	if got > 0.5 {
		t.Errorf("missing keywords: score = %v, want <= 0.5", got)
	}
}

func TestKeywordsScoreMinimum(t *testing.T) {
	ref := "Le **chat** aime le **chien**."
	answer := "chat mais pas l autre"
	got := keywordsScore(ref, answer)
	got2 := keywordsScore(ref, "chat et chien")
	if got >= got2 {
		t.Errorf("partial match should score lower than full match: %v >= %v", got, got2)
	}
}

func ollamaTestServer(t *testing.T, accuracy float64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner, _ := json.Marshal(map[string]float64{"accuracy": accuracy})
		resp := map[string]any{
			"message": map[string]string{"content": string(inner)},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestEvaluateCorrect(t *testing.T) {
	srv := ollamaTestServer(t, 0.9)
	defer srv.Close()

	cfg := evalConfig{
		ollamaURL: srv.URL,
		threshold: 0.7,
	}
	ev := newEvaluator(cfg)
	result, err := ev.evaluate(
		"Photosynthèse",
		"La **photosynthèse** convertit la lumière en énergie.",
		"photosynthèse convertit lumière en énergie",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.correct {
		t.Errorf("expected correct=true, got accuracy=%v keywordsScore=%v", result.accuracy, result.keywordsScore)
	}
}

func TestEvaluateIncorrectLowAccuracy(t *testing.T) {
	srv := ollamaTestServer(t, 0.2)
	defer srv.Close()

	cfg := evalConfig{
		ollamaURL: srv.URL,
		threshold: 0.7,
	}
	ev := newEvaluator(cfg)
	result, err := ev.evaluate("Concept", "**mot clé**", "mot clé")
	if err != nil {
		t.Fatal(err)
	}
	if result.correct {
		t.Errorf("expected correct=false for low LLM accuracy")
	}
}

func TestEvaluateOllamaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := evalConfig{ollamaURL: srv.URL, threshold: 0.7}
	ev := newEvaluator(cfg)
	_, err := ev.evaluate("Concept", "Reference", "Answer")
	if err == nil {
		t.Error("expected error on 500 response")
	}
}

func TestEvaluateOllamaMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message": {"content": "not-json"}}`))
	}))
	defer srv.Close()

	cfg := evalConfig{ollamaURL: srv.URL, threshold: 0.7}
	ev := newEvaluator(cfg)
	_, err := ev.evaluate("Concept", "Reference", "Answer")
	if err == nil {
		t.Error("expected error on malformed inner JSON")
	}
}
