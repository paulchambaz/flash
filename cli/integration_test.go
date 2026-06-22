package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupDeck writes a deck.md, parses it, opens a DB, and syncs the cards.
// Returns the DB and the path to the deck file.
func setupDeck(t *testing.T, content string) (*DB, string) {
	t.Helper()
	dir := t.TempDir()
	deckPath := filepath.Join(dir, "deck.md")
	dbPath := filepath.Join(dir, "deck.db")

	if err := os.WriteFile(deckPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cards, err := parseDeck(deckPath)
	if err != nil {
		t.Fatalf("parseDeck: %v", err)
	}

	db, err := openDB(dbPath, 24*time.Hour)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() { db.close() })

	if err := db.syncCards("deck", cards); err != nil {
		t.Fatalf("syncCards: %v", err)
	}
	return db, deckPath
}

// reloadDeck rewrites the deck file and re-syncs the DB.
func reloadDeck(t *testing.T, db *DB, deckPath, content string) {
	t.Helper()
	if err := os.WriteFile(deckPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cards, err := parseDeck(deckPath)
	if err != nil {
		t.Fatalf("parseDeck: %v", err)
	}
	if err := db.syncCards("deck", cards); err != nil {
		t.Fatalf("syncCards: %v", err)
	}
}

const initialDeck = `Photosynthèse
La **photosynthèse** convertit la lumière solaire en **énergie** chimique.

Mitose
La **mitose** est une division cellulaire produisant deux cellules **identiques**.

Méiose
La **méiose** produit quatre cellules **haploïdes** génétiquement distinctes.
`

// simulateReviews does one review per due card.
func simulateReviews(t *testing.T, db *DB, correct bool) {
	t.Helper()
	due, err := db.dueCards("deck")
	if err != nil {
		t.Fatalf("dueCards: %v", err)
	}
	for _, c := range due {
		if _, err := db.submitReview(c.ID, correct, 0.9, 0.9); err != nil {
			t.Fatalf("submitReview card %d: %v", c.ID, err)
		}
	}
}

func TestIntegrationAddCard(t *testing.T) {
	db, deckPath := setupDeck(t, initialDeck)

	// Review all 3 initial cards
	simulateReviews(t, db, true)

	total, _ := db.deckTotal("deck")
	if total != 3 {
		t.Fatalf("total = %d, want 3 before add", total)
	}

	// No cards should be due immediately after reviews
	due, _ := db.dueCards("deck")
	if len(due) != 0 {
		t.Fatalf("expected 0 due after reviews, got %d", len(due))
	}

	// Add a 4th card to the deck
	reloadDeck(t, db, deckPath, initialDeck+`
ADN
L'**ADN** est le support de l'**information génétique**.
`)

	total, _ = db.deckTotal("deck")
	if total != 4 {
		t.Errorf("total = %d, want 4 after add", total)
	}

	// Only the new card should be due (3 existing cards keep their scheduling)
	due, _ = db.dueCards("deck")
	if len(due) != 1 {
		t.Errorf("expected 1 due (new card only), got %d", len(due))
	}
	if due[0].Concept != "ADN" {
		t.Errorf("due card = %q, want %q", due[0].Concept, "ADN")
	}
	if due[0].Reps != 0 {
		t.Errorf("new card Reps = %d, want 0", due[0].Reps)
	}

	// Existing reviewed cards preserve their state
	stats, _ := db.deckStats("deck")
	if stats.New != 1 {
		t.Errorf("New = %d, want 1 (only the added card)", stats.New)
	}
}

func TestIntegrationModifyCard(t *testing.T) {
	db, deckPath := setupDeck(t, initialDeck)

	// Review all cards
	simulateReviews(t, db, true)

	due, _ := db.dueCards("deck")
	if len(due) != 0 {
		t.Fatalf("expected 0 due after reviews, got %d", len(due))
	}

	// Modify the Mitose card reference
	modified := `Photosynthèse
La **photosynthèse** convertit la lumière solaire en **énergie** chimique.

Mitose
La **mitose** est une division cellulaire produisant deux cellules **identiques**. Elle comprend les phases **prophase**, **métaphase**, **anaphase** et **télophase**.

Méiose
La **méiose** produit quatre cellules **haploïdes** génétiquement distinctes.
`
	reloadDeck(t, db, deckPath, modified)

	// Only Mitose should be due again (reference changed → reset)
	due, _ = db.dueCards("deck")
	if len(due) != 1 {
		t.Errorf("expected 1 due (modified card only), got %d", len(due))
	}
	if due[0].Concept != "Mitose" {
		t.Errorf("due card = %q, want %q", due[0].Concept, "Mitose")
	}
	if due[0].Reps != 0 {
		t.Errorf("modified card Reps = %d, want 0", due[0].Reps)
	}
	if due[0].DueDate != nil {
		t.Errorf("modified card DueDate should be nil, got %v", due[0].DueDate)
	}

	// Unmodified cards keep their state
	total, _ := db.deckTotal("deck")
	if total != 3 {
		t.Errorf("total = %d, want 3 (no cards added or removed)", total)
	}
	stats, _ := db.deckStats("deck")
	if stats.New != 1 {
		t.Errorf("New = %d, want 1 (only the modified card)", stats.New)
	}
}

func TestIntegrationRemoveCard(t *testing.T) {
	db, deckPath := setupDeck(t, initialDeck)

	// Review all cards
	simulateReviews(t, db, true)

	// Remove Méiose from the deck
	withoutMeiose := `Photosynthèse
La **photosynthèse** convertit la lumière solaire en **énergie** chimique.

Mitose
La **mitose** est une division cellulaire produisant deux cellules **identiques**.
`
	reloadDeck(t, db, deckPath, withoutMeiose)

	total, _ := db.deckTotal("deck")
	if total != 2 {
		t.Errorf("total = %d, want 2 after remove", total)
	}

	// No cards should be due (2 remaining cards keep their scheduling)
	due, _ := db.dueCards("deck")
	if len(due) != 0 {
		t.Errorf("expected 0 due after remove, got %d (%v)", len(due), due)
	}

	// Stats reflect only the 2 remaining cards
	stats, _ := db.deckStats("deck")
	if stats.Total != 2 {
		t.Errorf("stats.Total = %d, want 2", stats.Total)
	}
}

func TestIntegrationCombined(t *testing.T) {
	db, deckPath := setupDeck(t, initialDeck)

	// Partial reviews: only review Photosynthèse and Mitose
	due, _ := db.dueCards("deck")
	for _, c := range due {
		if c.Concept != "Méiose" {
			db.submitReview(c.ID, true, 0.9, 0.9)
		}
	}

	// Simultaneously: add ADN, modify Méiose, remove Mitose
	v2 := `Photosynthèse
La **photosynthèse** convertit la lumière solaire en **énergie** chimique.

Méiose
La **méiose** produit quatre cellules **haploïdes**. Contrairement à la mitose, elle comporte deux divisions successives.

ADN
L'**ADN** est le support de l'**information génétique**.
`
	reloadDeck(t, db, deckPath, v2)

	total, _ := db.deckTotal("deck")
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	due, _ = db.dueCards("deck")
	dueNames := make(map[string]bool, len(due))
	for _, c := range due {
		dueNames[c.Concept] = true
	}

	// Méiose: was not reviewed + reference changed → due
	if !dueNames["Méiose"] {
		t.Error("Méiose should be due (not reviewed + modified)")
	}
	// ADN: new card → due
	if !dueNames["ADN"] {
		t.Error("ADN should be due (new card)")
	}
	// Photosynthèse: reviewed, unchanged → not due
	if dueNames["Photosynthèse"] {
		t.Error("Photosynthèse should not be due (reviewed, unchanged)")
	}
	// Mitose: removed → not present at all
	if dueNames["Mitose"] {
		t.Error("Mitose should not exist (removed)")
	}

	if len(due) != 2 {
		t.Errorf("expected 2 due cards, got %d", len(due))
	}
}
