package main

import (
	"math"
	"testing"
	"time"
)

func TestRetention(t *testing.T) {
	tests := []struct {
		t, s float64
		want float64
	}{
		{0, 1, 1.0},
		{0, 0, 0.0},
		{1, 1, desiredRetention},
		{2, 2, desiredRetention},
	}
	for _, tt := range tests {
		got := retention(tt.t, tt.s)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("retention(%v,%v) = %v, want %v", tt.t, tt.s, got, tt.want)
		}
	}
}

func TestInitialSD(t *testing.T) {
	t.Run("incorrect always returns fsrsW[0] stability", func(t *testing.T) {
		s, _ := initialSD(false, 0.5)
		if math.Abs(s-fsrsW[0]) > 1e-9 {
			t.Errorf("stability = %v, want %v", s, fsrsW[0])
		}
	})
	t.Run("correct with accuracy=0 gives lower stability", func(t *testing.T) {
		s0, _ := initialSD(true, 0.0)
		s1, _ := initialSD(true, 1.0)
		if s0 >= s1 {
			t.Errorf("expected s(acc=0) < s(acc=1), got %v >= %v", s0, s1)
		}
	})
	t.Run("difficulty clamped to [1,10]", func(t *testing.T) {
		for _, acc := range []float64{0.0, 0.25, 0.5, 0.75, 1.0} {
			_, d := initialSD(true, acc)
			if d < 1 || d > 10 {
				t.Errorf("difficulty %v out of [1,10] for acc=%v", d, acc)
			}
		}
	})
}

func TestScheduleNewCard(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	state := CardState{Reps: 0}

	t.Run("correct new card with 1d step: 1-day interval, no reshow", func(t *testing.T) {
		sr := schedule(state, true, 1.0, 24*time.Hour, now)
		if sr.reshowInSession {
			t.Error("expected reshowInSession=false")
		}
		if math.Abs(sr.intervalDays-1.0) > 1e-9 {
			t.Errorf("intervalDays = %v, want 1.0", sr.intervalDays)
		}
		want := now.Add(24 * time.Hour)
		if !sr.nextDue.Equal(want) {
			t.Errorf("nextDue = %v, want %v", sr.nextDue, want)
		}
	})

	t.Run("incorrect new card with 1min step: reshow in session", func(t *testing.T) {
		sr := schedule(state, false, 0.0, time.Minute, now)
		if !sr.reshowInSession {
			t.Error("expected reshowInSession=true (T/10=6s floors to 1min ≤ 5min)")
		}
	})

	t.Run("incorrect new card with 1d step: no reshow", func(t *testing.T) {
		sr := schedule(state, false, 0.0, 24*time.Hour, now)
		if sr.reshowInSession {
			t.Error("expected reshowInSession=false (T/10=2.4h > 5min)")
		}
	})
}

func TestScheduleRep1(t *testing.T) {
	now := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)
	lastReview := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	s0, d0 := initialSD(true, 1.0)
	state := CardState{Reps: 1, Stability: s0, Difficulty: d0, LastReview: &lastReview}

	t.Run("rep 1 correct with 1d step: interval ≥ 1 day", func(t *testing.T) {
		sr := schedule(state, true, 1.0, 24*time.Hour, now)
		if sr.reshowInSession {
			t.Error("expected reshowInSession=false")
		}
		if sr.intervalDays < 1.0 {
			t.Errorf("intervalDays = %v, want ≥ 1.0", sr.intervalDays)
		}
	})

	t.Run("rep 1 incorrect with 1min step: reshow", func(t *testing.T) {
		sr := schedule(state, false, 0.0, time.Minute, now)
		if !sr.reshowInSession {
			t.Error("expected reshowInSession=true (reps<2, T/10=6s→1min ≤ 5min)")
		}
	})

	t.Run("rep 1 incorrect with 1d step: no reshow", func(t *testing.T) {
		sr := schedule(state, false, 0.0, 24*time.Hour, now)
		if sr.reshowInSession {
			t.Error("expected reshowInSession=false (reps<2, T/10=2.4h > 5min)")
		}
	})
}

func TestScheduleRep2Plus(t *testing.T) {
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	lastReview := time.Date(2025, 1, 7, 12, 0, 0, 0, time.UTC)
	state := CardState{Reps: 2, Stability: 5.0, Difficulty: 5.0, LastReview: &lastReview}

	t.Run("rep 2+ correct with 1d step: interval ≥ 1d", func(t *testing.T) {
		sr := schedule(state, true, 1.0, 24*time.Hour, now)
		if sr.reshowInSession {
			t.Error("unexpected reshowInSession=true")
		}
		if sr.intervalDays < 1.0 {
			t.Errorf("intervalDays %v < 1", sr.intervalDays)
		}
	})

	t.Run("larger step → proportionally larger interval", func(t *testing.T) {
		sr1 := schedule(state, true, 1.0, 12*time.Hour, now)
		sr2 := schedule(state, true, 1.0, 24*time.Hour, now)
		if sr1.intervalDays >= sr2.intervalDays {
			t.Errorf("expected smaller step → shorter interval: %v >= %v", sr1.intervalDays, sr2.intervalDays)
		}
	})

	t.Run("rep 2 incorrect with 1min step: reshow", func(t *testing.T) {
		sr := schedule(state, false, 0.0, time.Minute, now)
		if !sr.reshowInSession {
			t.Error("expected reshowInSession=true (reps≥2, step=1min ≤ 5min)")
		}
	})

	t.Run("rep 2 incorrect with 1d step: no reshow", func(t *testing.T) {
		sr := schedule(state, false, 0.0, 24*time.Hour, now)
		if sr.reshowInSession {
			t.Error("expected reshowInSession=false (reps≥2, step=1d > 5min)")
		}
	})
}

func TestScheduleRep3IncorrectNoReshow(t *testing.T) {
	now := time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC)
	lastReview := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	state := CardState{Reps: 3, Stability: 5.0, Difficulty: 5.0, LastReview: &lastReview}

	sr := schedule(state, false, 0.0, 24*time.Hour, now)
	if sr.reshowInSession {
		t.Error("reps=3 incorrect with 1d step: expected reshowInSession=false")
	}
}

func TestScheduleDifficultyBounds(t *testing.T) {
	now := time.Now()
	last := now.Add(-24 * time.Hour)
	state := CardState{Reps: 5, Stability: 3.0, Difficulty: 5.0, LastReview: &last}

	for _, acc := range []float64{0.0, 0.5, 1.0} {
		for _, correct := range []bool{true, false} {
			sr := schedule(state, correct, acc, 24*time.Hour, now)
			if sr.difficulty < 1 || sr.difficulty > 10 {
				t.Errorf("difficulty %v out of [1,10] (correct=%v acc=%v)", sr.difficulty, correct, acc)
			}
			if sr.stability < 0.1 {
				t.Errorf("stability %v < 0.1 (correct=%v acc=%v)", sr.stability, correct, acc)
			}
		}
	}
}
