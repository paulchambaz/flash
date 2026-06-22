package main

import (
	"math"
	"time"
)

var fsrsW = [21]float64{
	0.40255, 1.18385, 3.1262, 15.4722,
	7.2102,
	0.5316,
	1.0651,
	0.0589,
	1.469, 0.166, 0.9734,
	1.9214, 0.11, 2.9898, 0.29,
	2.2700, 2.9898,
	0.51, 0.8, 0.1, 0.9,
}

const desiredRetention = 0.9

// Extended params map A ∈ [0,1] to FSRS grade behavior.
// h_succ(0)≈0.8 (Hard), h_succ(1)≈1.3 (Easy); φ: -1→+1 for success, -2 for failure.
type extParams struct {
	u0, u1 float64 // h_succ(A) = exp(u0 + u1*A)
	v0, v1 float64 // h_fail(A) = exp(v0 + v1*A)
	a0, a1 float64 // φ(C=true,  A) = a0 + a1*A
	b0, b1 float64 // φ(C=false, A) = b0 + b1*A
}

var defaultExtParams = extParams{
	u0: math.Log(0.8),
	u1: math.Log(1.3) - math.Log(0.8),
	v0: 0, v1: 0,
	a0: -1, a1: 2,
	b0: -2, b1: 0,
}

type schedResult struct {
	stability       float64
	difficulty      float64
	intervalDays    float64
	nextDue         time.Time
	reshowInSession bool // true = card comes back in the current session
}

func schedule(state CardState, correct bool, accuracy float64, step time.Duration, now time.Time) schedResult {
	stepDays := step.Hours() / 24.0
	ep := defaultExtParams
	var sNew, dNew float64

	if state.Reps == 0 {
		sNew, dNew = initialSD(correct, accuracy)
	} else {
		var daysSinceLast float64
		if state.LastReview != nil {
			daysSinceLast = now.Sub(*state.LastReview).Hours() / 24
		}
		r := retention(daysSinceLast, state.Stability)
		s, d := state.Stability, state.Difficulty

		if correct {
			hSucc := math.Exp(ep.u0 + ep.u1*accuracy)
			sNew = s * (1 + math.Exp(fsrsW[8])*(11-d)*math.Pow(s, -fsrsW[9])*(math.Exp(fsrsW[10]*(1-r))-1)*hSucc)
		} else {
			hFail := math.Exp(ep.v0 + ep.v1*accuracy)
			sNew = fsrsW[11] * math.Pow(d, -fsrsW[12]) * (math.Pow(s+1, fsrsW[13]) - 1) * math.Exp(fsrsW[14]*(1-r)) * hFail
			sNew = math.Min(sNew, s)
		}
		sNew = math.Max(0.1, sNew)

		phi := ep.a0 + ep.a1*accuracy
		if !correct {
			phi = ep.b0 + ep.b1*accuracy
		}
		dNew = fsrsW[7]*fsrsW[4] + (1-fsrsW[7])*(d-fsrsW[6]*phi)
		dNew = math.Max(1, math.Min(10, dNew))
	}

	if !correct {
		if state.Reps < 2 {
			// Learning failure: T/10, floor 1 min.
			again := step / 10
			if again < time.Minute {
				again = time.Minute
			}
			return schedResult{
				stability:       sNew,
				difficulty:      dNew,
				intervalDays:    again.Hours() / 24.0,
				nextDue:         now.Add(again),
				reshowInSession: again <= 5*time.Minute,
			}
		}
		// Review failure: re-learn in T.
		return schedResult{
			stability:       sNew,
			difficulty:      dNew,
			intervalDays:    stepDays,
			nextDue:         now.Add(step),
			reshowInSession: step <= 5*time.Minute,
		}
	}

	var intervalDays float64
	if state.Reps == 0 {
		// First correct: T.
		intervalDays = stepDays
	} else {
		// Graduated: FSRS stability × T, minimum T.
		intervalDays = math.Max(stepDays, sNew*stepDays)
	}

	return schedResult{
		stability:    sNew,
		difficulty:   dNew,
		intervalDays: intervalDays,
		nextDue:      now.Add(time.Duration(intervalDays * float64(24*time.Hour))),
	}
}

func retention(t, s float64) float64 {
	if s <= 0 {
		return 0
	}
	return math.Exp(math.Log(desiredRetention) * t / s)
}

func initialSD(correct bool, accuracy float64) (stability, difficulty float64) {
	if !correct {
		return fsrsW[0], fsrsW[4]
	}
	stability = fsrsW[1] + (fsrsW[3]-fsrsW[1])*accuracy
	gEff := 1 + 2*accuracy
	difficulty = fsrsW[4] - math.Exp(fsrsW[5]*gEff) + 1
	difficulty = math.Max(1, math.Min(10, difficulty))
	return
}
