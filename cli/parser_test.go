package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempDeck(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseDeckSingleCard(t *testing.T) {
	path := writeTempDeck(t, "Concept A\nReference line 1\n")
	cards, err := parseDeck(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("len(cards) = %d, want 1", len(cards))
	}
	if cards[0].Concept != "Concept A" {
		t.Errorf("Concept = %q, want %q", cards[0].Concept, "Concept A")
	}
	if cards[0].Reference != "Reference line 1" {
		t.Errorf("Reference = %q, want %q", cards[0].Reference, "Reference line 1")
	}
}

func TestParseDeckMultipleCards(t *testing.T) {
	content := "Concept A\nRef A\n\nConcept B\nRef B\n"
	cards, err := parseDeck(writeTempDeck(t, content))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("len(cards) = %d, want 2", len(cards))
	}
	if cards[1].Concept != "Concept B" {
		t.Errorf("cards[1].Concept = %q", cards[1].Concept)
	}
}

func TestParseDeckMultilineReference(t *testing.T) {
	content := "Question\nLine 1\nLine 2\nLine 3\n"
	cards, err := parseDeck(writeTempDeck(t, content))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("len(cards) = %d, want 1", len(cards))
	}
	want := "Line 1\nLine 2\nLine 3"
	if cards[0].Reference != want {
		t.Errorf("Reference = %q, want %q", cards[0].Reference, want)
	}
}

func TestParseDeckWhitespaceTrimmed(t *testing.T) {
	content := "  Concept  \n  Reference  \n"
	cards, err := parseDeck(writeTempDeck(t, content))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("len(cards) = %d, want 1", len(cards))
	}
	if cards[0].Concept != "Concept" {
		t.Errorf("Concept = %q", cards[0].Concept)
	}
	if cards[0].Reference != "Reference" {
		t.Errorf("Reference = %q", cards[0].Reference)
	}
}

func TestParseDeckEmptyBlocksSkipped(t *testing.T) {
	content := "\n\nConcept A\nRef A\n\n\n\nConcept B\nRef B\n\n"
	cards, err := parseDeck(writeTempDeck(t, content))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("len(cards) = %d, want 2", len(cards))
	}
}

func TestParseDeckConceptOnlySkipped(t *testing.T) {
	content := "ConceptOnly\n\nConcept B\nRef B\n"
	cards, err := parseDeck(writeTempDeck(t, content))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("len(cards) = %d, want 1 (concept-only block skipped)", len(cards))
	}
	if cards[0].Concept != "Concept B" {
		t.Errorf("Concept = %q", cards[0].Concept)
	}
}

func TestParseDeckFileNotFound(t *testing.T) {
	_, err := parseDeck("/nonexistent/path/deck.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseDeckEmpty(t *testing.T) {
	cards, err := parseDeck(writeTempDeck(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 0 {
		t.Errorf("len(cards) = %d, want 0", len(cards))
	}
}
