package main

import (
	"database/sql"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := openDB(":memory:", 24*time.Hour)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() { db.close() })
	return db
}

func TestMigrateIdempotent(t *testing.T) {
	db := newTestDB(t)
	if err := db.migrate(); err != nil {
		t.Errorf("second migrate() = %v", err)
	}
}

func TestSyncCardsInsert(t *testing.T) {
	db := newTestDB(t)
	cards := []Card{
		{Concept: "A", Reference: "Ref A"},
		{Concept: "B", Reference: "Ref B"},
	}
	if err := db.syncCards("deck", cards); err != nil {
		t.Fatal(err)
	}
	n, err := db.deckTotal("deck")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("deckTotal = %d, want 2", n)
	}
}

func TestSyncCardsDeleteRemoved(t *testing.T) {
	db := newTestDB(t)
	cards := []Card{
		{Concept: "A", Reference: "Ref A"},
		{Concept: "B", Reference: "Ref B"},
	}
	if err := db.syncCards("deck", cards); err != nil {
		t.Fatal(err)
	}

	// Remove B
	if err := db.syncCards("deck", cards[:1]); err != nil {
		t.Fatal(err)
	}
	n, _ := db.deckTotal("deck")
	if n != 1 {
		t.Errorf("deckTotal after delete = %d, want 1", n)
	}
}

func TestSyncCardsRefHashChange(t *testing.T) {
	db := newTestDB(t)
	if err := db.syncCards("deck", []Card{{Concept: "A", Reference: "old"}}); err != nil {
		t.Fatal(err)
	}

	// Submit a review so reps > 0 and due_date is in the future
	due, _ := db.dueCards("deck")
	if len(due) == 0 {
		t.Fatal("expected 1 due card")
	}
	cardID := due[0].ID
	db.submitReview(cardID, true, 1.0, 1.0)

	// Verify card is no longer immediately due
	due2, _ := db.dueCards("deck")
	if len(due2) != 0 {
		t.Fatal("card should not be due immediately after review")
	}

	// Modify reference → card should be fully reset and due again
	if err := db.syncCards("deck", []Card{{Concept: "A", Reference: "new content"}}); err != nil {
		t.Fatal(err)
	}

	card, err := db.getCard(cardID)
	if err != nil {
		t.Fatal(err)
	}
	if card.Reference != "new content" {
		t.Errorf("Reference = %q, want %q", card.Reference, "new content")
	}
	if card.Reps != 0 {
		t.Errorf("Reps = %d, want 0 after ref change", card.Reps)
	}
	if card.DueDate != nil {
		t.Errorf("DueDate should be nil after ref change, got %v", card.DueDate)
	}
	if card.LastReview != nil {
		t.Errorf("LastReview should be nil after ref change, got %v", card.LastReview)
	}

	// Card is immediately due again (due_date IS NULL)
	due3, _ := db.dueCards("deck")
	if len(due3) != 1 {
		t.Errorf("expected card to be due again after ref change, got %d due cards", len(due3))
	}
}

func TestDueCards(t *testing.T) {
	db := newTestDB(t)
	if err := db.syncCards("deck", []Card{
		{Concept: "A", Reference: "Ref A"},
		{Concept: "B", Reference: "Ref B"},
	}); err != nil {
		t.Fatal(err)
	}

	due, err := db.dueCards("deck")
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 2 {
		t.Errorf("dueCards = %d, want 2 (new cards with NULL due_date are always due)", len(due))
	}
}

func TestSubmitReviewUpdatesCard(t *testing.T) {
	db := newTestDB(t)
	if err := db.syncCards("deck", []Card{{Concept: "A", Reference: "Ref A"}}); err != nil {
		t.Fatal(err)
	}
	due, _ := db.dueCards("deck")
	id := due[0].ID

	sr, err := db.submitReview(id, true, 1.0, 1.0)
	if err != nil {
		t.Fatalf("submitReview: %v", err)
	}
	if sr.intervalDays == 0 {
		t.Error("expected non-zero intervalDays")
	}

	card, err := db.getCard(id)
	if err != nil {
		t.Fatal(err)
	}
	if card.Reps != 1 {
		t.Errorf("Reps = %d, want 1", card.Reps)
	}
	if card.LastReview == nil {
		t.Error("LastReview should not be nil")
	}
	if card.DueDate == nil {
		t.Error("DueDate should not be nil")
	}
}

func TestResetDeck(t *testing.T) {
	db := newTestDB(t)
	if err := db.syncCards("deck", []Card{{Concept: "A", Reference: "Ref A"}}); err != nil {
		t.Fatal(err)
	}
	due, _ := db.dueCards("deck")
	db.submitReview(due[0].ID, true, 1.0, 1.0)

	if err := db.resetDeck("deck"); err != nil {
		t.Fatal(err)
	}

	card, _ := db.getCard(due[0].ID)
	if card.Reps != 0 {
		t.Errorf("Reps after reset = %d, want 0", card.Reps)
	}
	if card.DueDate != nil {
		t.Error("DueDate should be nil after reset")
	}
}

func TestLastReviewTimeScanNullTime(t *testing.T) {
	db := newTestDB(t)
	if err := db.syncCards("deck", []Card{{Concept: "A", Reference: "Ref A"}}); err != nil {
		t.Fatal(err)
	}

	// No reviews yet
	lt, err := db.lastReviewTime("deck")
	if err != nil {
		t.Fatalf("lastReviewTime with no reviews: %v", err)
	}
	if lt != nil {
		t.Errorf("expected nil, got %v", lt)
	}

	// Submit a review and check the timestamp
	due, _ := db.dueCards("deck")
	before := time.Now().Add(-time.Second)
	db.submitReview(due[0].ID, true, 1.0, 1.0)
	after := time.Now().Add(time.Second)

	lt, err = db.lastReviewTime("deck")
	if err != nil {
		t.Fatalf("lastReviewTime after review: %v", err)
	}
	if lt == nil {
		t.Fatal("expected non-nil lastReviewTime")
	}
	if lt.Before(before) || lt.After(after) {
		t.Errorf("lastReviewTime %v out of expected range [%v, %v]", lt, before, after)
	}
}

func TestDeckStats(t *testing.T) {
	db := newTestDB(t)
	if err := db.syncCards("deck", []Card{
		{Concept: "A", Reference: "Ref A"},
		{Concept: "B", Reference: "Ref B"},
	}); err != nil {
		t.Fatal(err)
	}

	stats, err := db.deckStats("deck")
	if err != nil {
		t.Fatalf("deckStats: %v", err)
	}
	if stats.Total != 2 {
		t.Errorf("Total = %d, want 2", stats.Total)
	}
	if stats.New != 2 {
		t.Errorf("New = %d, want 2 (no reviews yet)", stats.New)
	}
	if stats.LastReview != nil {
		t.Errorf("LastReview should be nil, got %v", stats.LastReview)
	}

	// Submit a review and check LastReview is populated (validates scanNullTime fix)
	due, _ := db.dueCards("deck")
	db.submitReview(due[0].ID, true, 1.0, 1.0)
	db.submitReview(due[0].ID, false, 0.3, 0.3)

	stats2, err := db.deckStats("deck")
	if err != nil {
		t.Fatalf("deckStats after reviews: %v", err)
	}
	if stats2.LastReview == nil {
		t.Error("LastReview should not be nil after review")
	}
	if stats2.ReviewCount == 0 {
		t.Error("ReviewCount should be > 0")
	}
}

func TestScanNullTimeGoStringFormat(t *testing.T) {
	// Go's time.Time.String() format is what modernc.org/sqlite stores when
	// a time.Time is passed as a query parameter (pre-fix behavior).
	cases := []struct {
		s    string
		want string // expected RFC3339 output
	}{
		{"2026-06-22 09:19:31.069793657 +0000 UTC", "2026-06-22T09:19:31.069793657Z"},
		{"2026-06-22 09:12:41.194191739 +0000 UTC", "2026-06-22T09:12:41.194191739Z"},
		{"2025-12-31 23:59:59 +0000 UTC", "2025-12-31T23:59:59Z"},
	}
	for _, c := range cases {
		ns := sql.NullString{String: c.s, Valid: true}
		got, err := scanNullTime(ns)
		if err != nil {
			t.Errorf("scanNullTime(%q) error: %v", c.s, err)
			continue
		}
		if got == nil {
			t.Errorf("scanNullTime(%q) = nil, want %q", c.s, c.want)
			continue
		}
		if got.UTC().Format(time.RFC3339Nano) != c.want {
			t.Errorf("scanNullTime(%q) = %q, want %q", c.s, got.UTC().Format(time.RFC3339Nano), c.want)
		}
	}

	// NULL → nil, no error
	ns := sql.NullString{Valid: false}
	got, err := scanNullTime(ns)
	if err != nil || got != nil {
		t.Errorf("scanNullTime(NULL) = %v, %v; want nil, nil", got, err)
	}
}

func TestGetCardRoundtrip(t *testing.T) {
	db := newTestDB(t)
	if err := db.syncCards("deck", []Card{{Concept: "Q", Reference: "R"}}); err != nil {
		t.Fatal(err)
	}
	due, _ := db.dueCards("deck")
	id := due[0].ID

	card, err := db.getCard(id)
	if err != nil {
		t.Fatal(err)
	}
	if card.Concept != "Q" {
		t.Errorf("Concept = %q, want %q", card.Concept, "Q")
	}
	if card.Reference != "R" {
		t.Errorf("Reference = %q, want %q", card.Reference, "R")
	}
	if card.Reps != 0 {
		t.Errorf("Reps = %d, want 0", card.Reps)
	}
}
