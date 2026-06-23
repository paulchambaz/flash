package main

import (
	"math"
	"testing"
	"time"
)

var testPace = 7 * 24 * time.Hour

func TestScheduleFirstReview(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	state := CardState{}

	t.Run("correct high acc reshown", func(t *testing.T) {
		sr := schedule(state, nil, true, 1.0, testPace, now)
		if !sr.reshowInSession {
			t.Errorf("want reshown, got interval %v", sr.nextDue.Sub(now))
		}
	})

	t.Run("incorrect reshown", func(t *testing.T) {
		sr := schedule(state, nil, false, 0.0, testPace, now)
		if !sr.reshowInSession {
			t.Error("want reshown for wrong answer")
		}
	})

	t.Run("stability at least hMinMinutes", func(t *testing.T) {
		for _, acc := range []float64{0.0, 0.5, 1.0} {
			for _, correct := range []bool{true, false} {
				sr := schedule(state, nil, correct, acc, testPace, now)
				if sr.stability < hMinMinutes {
					t.Errorf("acc=%.1f correct=%v: stability %v < hMin", acc, correct, sr.stability)
				}
			}
		}
	})

	t.Run("higher accuracy longer interval", func(t *testing.T) {
		sr0 := schedule(state, nil, true, 0.0, testPace, now)
		sr1 := schedule(state, nil, true, 1.0, testPace, now)
		if sr0.stability >= sr1.stability {
			t.Errorf("acc=0 stability %v >= acc=1 stability %v", sr0.stability, sr1.stability)
		}
	})
}

func TestScheduleGrowth(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	state := CardState{}

	t.Run("more correct history grows stability", func(t *testing.T) {
		sr0 := schedule(state, nil, true, 0.9, testPace, now)

		history := []reviewPoint{
			{dtMinutes: 0, correct: true, accuracy: 0.9},
			{dtMinutes: 30, correct: true, accuracy: 0.9},
		}
		sr2 := schedule(state, history, true, 0.9, testPace, now)
		if sr2.stability <= sr0.stability {
			t.Errorf("richer history didn't grow stability: %v <= %v", sr2.stability, sr0.stability)
		}
	})

	t.Run("wrong answer gives lower stability than correct", func(t *testing.T) {
		history := []reviewPoint{
			{dtMinutes: 0, correct: true, accuracy: 0.9},
			{dtMinutes: 60, correct: true, accuracy: 0.9},
		}
		srCorrect := schedule(state, history, true, 0.9, testPace, now)
		srWrong := schedule(state, history, false, 0.0, testPace, now)
		if srWrong.stability >= srCorrect.stability {
			t.Errorf("wrong stability %v >= correct %v", srWrong.stability, srCorrect.stability)
		}
	})

	t.Run("stability capped at pace", func(t *testing.T) {
		var history []reviewPoint
		for i := range 20 {
			history = append(history, reviewPoint{
				dtMinutes: float64(i * 1440),
				correct:   true,
				accuracy:  1.0,
			})
		}
		sr := schedule(state, history, true, 1.0, testPace, now)
		if sr.stability > testPace.Minutes()+1 {
			t.Errorf("stability %v exceeds pace %v min", sr.stability, testPace.Minutes())
		}
	})

	t.Run("larger pace scales intervals up", func(t *testing.T) {
		smallPace := 7 * 24 * time.Hour
		largePace := 30 * 24 * time.Hour
		sr1 := schedule(state, nil, true, 0.9, smallPace, now)
		sr2 := schedule(state, nil, true, 0.9, largePace, now)
		if sr2.stability <= sr1.stability {
			t.Errorf("larger pace should give larger interval: %v <= %v", sr2.stability, sr1.stability)
		}
	})

	t.Run("acc=0.7 correct progresses monotonically", func(t *testing.T) {
		var history []reviewPoint
		prev := 0.0
		for i := range 6 {
			sr := schedule(state, history, true, 0.7, testPace, now)
			if sr.stability < prev {
				t.Errorf("rep %d: stability decreased: %v < %v", i, sr.stability, prev)
			}
			prev = sr.stability
			history = append(history, reviewPoint{dtMinutes: sr.stability, correct: true, accuracy: 0.7})
		}
	})
}

func TestScheduleReshowFlag(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	state := CardState{}

	t.Run("reshowInSession matches interval <= reshowWindow", func(t *testing.T) {
		for _, acc := range []float64{0.0, 0.5, 1.0} {
			for _, correct := range []bool{true, false} {
				sr := schedule(state, nil, correct, acc, testPace, now)
				interval := sr.nextDue.Sub(now)
				wantReshow := interval <= reshowWindow
				if sr.reshowInSession != wantReshow {
					t.Errorf("acc=%.1f correct=%v: reshowInSession=%v but interval=%v",
						acc, correct, sr.reshowInSession, interval)
				}
			}
		}
	})

	t.Run("many correct reviews not reshown", func(t *testing.T) {
		var history []reviewPoint
		for i := range 10 {
			history = append(history, reviewPoint{
				dtMinutes: float64(i * 120),
				correct:   true,
				accuracy:  0.9,
			})
		}
		sr := schedule(state, history, true, 0.9, testPace, now)
		if sr.reshowInSession {
			t.Errorf("expected not reshown after many correct reviews, interval=%v", sr.nextDue.Sub(now))
		}
	})
}

func TestScheduleBounds(t *testing.T) {
	now := time.Now()
	state := CardState{}
	histories := [][]reviewPoint{
		nil,
		{{dtMinutes: 0, correct: true, accuracy: 0.9}},
		{
			{dtMinutes: 0, correct: true, accuracy: 0.9},
			{dtMinutes: 60, correct: true, accuracy: 0.8},
			{dtMinutes: 120, correct: false, accuracy: 0.3},
		},
	}
	for _, h := range histories {
		for _, acc := range []float64{0.0, 0.5, 1.0} {
			for _, correct := range []bool{true, false} {
				sr := schedule(state, h, correct, acc, testPace, now)
				if sr.stability < hMinMinutes {
					t.Errorf("stability %v < hMin", sr.stability)
				}
				if sr.stability > testPace.Minutes()+1 {
					t.Errorf("stability %v > pace", sr.stability)
				}
				if sr.nextDue.Before(now) {
					t.Error("nextDue in the past")
				}
			}
		}
	}
}

func TestReviewSignal(t *testing.T) {
	cases := []struct {
		correct  bool
		accuracy float64
		want     float64
	}{
		{true, 1.0, 1.0},
		{true, 0.5, 0.5},
		{true, 0.0, 0.0},
		{false, 0.0, -1.0},
		{false, 0.5, -0.5},
		{false, 1.0, 0.0},
	}
	for _, c := range cases {
		got := reviewSignal(reviewPoint{correct: c.correct, accuracy: c.accuracy})
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("correct=%v acc=%v: got %v, want %v", c.correct, c.accuracy, got, c.want)
		}
	}
}
