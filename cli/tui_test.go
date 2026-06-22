package main

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDateToday(t *testing.T) {
	now := time.Now()
	got := formatDate(now)
	if got != "today" {
		t.Errorf("formatDate(now) = %q, want %q", got, "today")
	}
}

func TestFormatDateTomorrow(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour)
	got := formatDate(tomorrow)
	if got != "tomorrow" {
		t.Errorf("formatDate(tomorrow) = %q, want %q", got, "tomorrow")
	}
}

func TestFormatDateThisYear(t *testing.T) {
	// Use a fixed date guaranteed to be this year and not today/tomorrow
	now := time.Now()
	target := time.Date(now.Year(), time.January, 15, 12, 0, 0, 0, time.Local)
	if target.YearDay() == now.YearDay() || target.YearDay() == now.YearDay()+1 {
		target = time.Date(now.Year(), time.December, 20, 12, 0, 0, 0, time.Local)
	}
	got := formatDate(target)
	if strings.Contains(got, string(rune('0'+target.Year()/1000))) {
		// should NOT contain year for same-year dates
		t.Errorf("same-year formatDate should not contain year: %q", got)
	}
	if !strings.Contains(got, enMonths[target.Month()]) {
		t.Errorf("formatDate = %q, expected month %q", got, enMonths[target.Month()])
	}
}

func TestFormatDateOtherYear(t *testing.T) {
	future := time.Date(2030, time.March, 5, 0, 0, 0, 0, time.Local)
	got := formatDate(future)
	if !strings.Contains(got, "2030") {
		t.Errorf("formatDate other year = %q, want year 2030", got)
	}
	if !strings.Contains(got, "March") {
		t.Errorf("formatDate = %q, want month 'March'", got)
	}
}

func TestRenderBoldMD(t *testing.T) {
	tests := []struct {
		in      string
		wantSub string // must be present in output
		noSub   string // must NOT be present in output
	}{
		{"**photosynthèse**", "photosynthèse", "**"},
		{"normal text", "normal text", ""},
		{"**a** and **b**", "and", "**"},
	}
	for _, tt := range tests {
		got := renderBoldMD(tt.in)
		if tt.wantSub != "" && !strings.Contains(got, tt.wantSub) {
			t.Errorf("renderBoldMD(%q) = %q, want substring %q", tt.in, got, tt.wantSub)
		}
		if tt.noSub != "" && strings.Contains(got, tt.noSub) {
			t.Errorf("renderBoldMD(%q) = %q, should not contain %q", tt.in, got, tt.noSub)
		}
	}
}

func TestRenderBoldMDEvalFound(t *testing.T) {
	ref := "La **photosynthèse** convertit la lumière."
	answer := "photosynthèse"
	got := renderBoldMDEval(ref, answer, 0.7)
	if strings.Contains(got, "**") {
		t.Errorf("renderBoldMDEval should remove ** markers, got %q", got)
	}
	if !strings.Contains(got, "photosynthèse") {
		t.Errorf("renderBoldMDEval should contain keyword text, got %q", got)
	}
}

func TestRenderBoldMDEvalMissing(t *testing.T) {
	ref := "La **photosynthèse** est importante."
	answer := "rien de pertinent"
	got := renderBoldMDEval(ref, answer, 0.7)
	if strings.Contains(got, "**") {
		t.Errorf("renderBoldMDEval should remove ** markers, got %q", got)
	}
}

func TestAnsiWordWrapNoop(t *testing.T) {
	s := "short"
	got := ansiWordWrap(s, 80)
	if got != s {
		t.Errorf("ansiWordWrap short string = %q, want %q", got, s)
	}
}

func TestAnsiWordWrapWraps(t *testing.T) {
	s := "word1 word2 word3 word4 word5"
	got := ansiWordWrap(s, 12)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("ansiWordWrap did not wrap: %q", got)
	}
}

func TestAnsiWordWrapPreservesNewlines(t *testing.T) {
	s := "line one\nline two"
	got := ansiWordWrap(s, 80)
	if !strings.Contains(got, "\n") {
		t.Errorf("ansiWordWrap should preserve existing newlines: %q", got)
	}
}

func TestAnsiWordWrapZeroWidth(t *testing.T) {
	s := "some text"
	got := ansiWordWrap(s, 0)
	if got != s {
		t.Errorf("ansiWordWrap with width=0 should return unchanged: %q", got)
	}
}

func TestWrapLineEmpty(t *testing.T) {
	got := wrapLine("", 40)
	if got != "" {
		t.Errorf("wrapLine empty = %q, want %q", got, "")
	}
}
