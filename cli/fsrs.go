package main

import (
	"math"
	"time"
)

// reshowWindow: cards scheduled within this duration come back in the same session.
const reshowWindow = time.Hour

// hMinMinutes: minimum half-life — floor for how quickly a card can reappear.
const hMinMinutes = 1.0

// defaultTheta: prior learning rate for a card with no history.
// At θ=0.28, roughly 4 correct answers at accuracy 0.9 bring a card from
// s=0 (new) to s=1 (mastered), regardless of pace.
const defaultTheta = 0.28

// thetaLambda: regularisation strength anchoring θ toward defaultTheta.
// When reviews are non-informative (always on-schedule), the prior dominates.
// When hard evidence exists (failures at short intervals, late reviews), data wins.
const thetaLambda = 0.2

type schedResult struct {
	stability       float64 // half-life in minutes: hMin × (pace/hMin)^s
	difficulty      float64 // per-card learning rate θ ∈ [0.01, 1]
	intervalDays    float64
	nextDue         time.Time
	reshowInSession bool
}

// reviewPoint holds the outcome of one review in a card's history.
type reviewPoint struct {
	dtMinutes float64 // minutes elapsed since the previous review (0 for first)
	correct   bool
	accuracy  float64 // ∈ [0, 1]
}

// schedule returns the next review interval using a log-scale Bayesian model.
//
// Normalized stability s ∈ [0, 1] maps to half-life h via:
//
//	h(s) = hMin × (pace/hMin)^s        (log-linear)
//
// s=0 → h=1 min (new card), s=1 → h=pace (fully mastered).
// Pace is a true scaling factor: same learning curve shape regardless of pace.
//
// Per-review update:
//
//	s_i = clip(s_{i−1} + θ · d_i, 0, 1)
//	d_i = accuracy_i          if correct   →  d ∈ [0, +1]
//	d_i = accuracy_i − 1      if incorrect →  d ∈ [−1,  0]
//
// θ is estimated from the full history by minimising a regularised loss:
//
//	L(θ) = Σ (c_i − r^{Δt_i/h(s_{i−1})})²  +  λ (log θ − log θ₀)²
//
// c_i ∈ {0,1} is the binary recall outcome (not the continuous accuracy).
// This separation is essential: using continuous acc in the prediction loss
// causes θ to collapse when accuracy < r, because the residual (acc − r) is
// constant across all θ when reviews happen on schedule (Δt ≈ h).
// The regularisation term anchors θ toward defaultTheta when data is
// non-informative (consistent on-schedule reviews), and yields to the data
// when failures or irregular timing provide genuine signal.
func schedule(state CardState, history []reviewPoint, correct bool, accuracy float64, pace time.Duration, now time.Time) schedResult {
	const r = 0.85

	hMax := pace.Minutes()
	if hMax < 1 {
		hMax = 7 * 24 * 60
	}

	// dt for the current review (0 if card never reviewed before).
	var dtCurrent float64
	if state.LastReview != nil {
		dtCurrent = now.Sub(*state.LastReview).Minutes()
	}

	// Append current review so θ estimation includes it.
	all := make([]reviewPoint, len(history)+1)
	copy(all, history)
	all[len(history)] = reviewPoint{dtMinutes: dtCurrent, correct: correct, accuracy: accuracy}

	// Need ≥ 2 reviews to compare prediction to outcome.
	theta := defaultTheta
	if len(all) >= 2 {
		theta = estimateTheta(all, r, hMax)
	}

	s := simulateS(theta, all)
	h := math.Max(hMinMinutes, math.Min(hMax, sToH(s, hMax)))
	interval := time.Duration(h * float64(time.Minute))

	return schedResult{
		stability:       h,
		difficulty:      theta,
		intervalDays:    interval.Hours() / 24.0,
		nextDue:         now.Add(interval),
		reshowInSession: interval <= reshowWindow,
	}
}

// sToH maps normalized stability s ∈ [0,1] to half-life in minutes.
// s=0 → hMin, s=1 → hMax, log-linear in between.
func sToH(s, hMax float64) float64 {
	return hMinMinutes * math.Pow(hMax/hMinMinutes, s)
}

// simulateS forward-simulates normalized stability for a given θ over history.
func simulateS(theta float64, history []reviewPoint) float64 {
	s := 0.0
	for _, p := range history {
		s = math.Max(0, math.Min(1, s+theta*reviewSignal(p)))
	}
	return s
}

// estimateTheta returns θ ∈ [0.01, 1] minimising the regularised loss
// over the full review history, via a 300-point log-scale grid search.
func estimateTheta(history []reviewPoint, r, hMax float64) float64 {
	const steps = 300
	logLo, logHi := math.Log(0.01), math.Log(1.0)

	best, bestLoss := defaultTheta, math.Inf(1)
	for i := 0; i <= steps; i++ {
		theta := math.Exp(logLo + float64(i)/steps*(logHi-logLo))
		if l := thetaLoss(theta, history, r, hMax); l < bestLoss {
			bestLoss = l
			best = theta
		}
	}
	return math.Max(0.01, math.Min(1.0, best))
}

// thetaLoss computes the regularised loss for a given θ:
//
//	L(θ) = Σ (c_i − r^{Δt_i/h(s_{i−1})})²  +  λ (log θ − log θ₀)²
//
// c_i ∈ {0,1} — binary recall outcome, not continuous accuracy.
func thetaLoss(theta float64, history []reviewPoint, r, hMax float64) float64 {
	s := 0.0
	var loss float64
	for _, p := range history {
		if p.dtMinutes > 0 {
			h := math.Max(hMinMinutes, math.Min(hMax, sToH(s, hMax)))
			pred := math.Pow(r, p.dtMinutes/h)
			c := 0.0
			if p.correct {
				c = 1.0
			}
			diff := c - pred
			loss += diff * diff
		}
		s = math.Max(0, math.Min(1, s+theta*reviewSignal(p)))
	}
	// Log-space regularisation: pulls θ toward defaultTheta when data is flat.
	logRatio := math.Log(theta) - math.Log(defaultTheta)
	loss += thetaLambda * logRatio * logRatio
	return loss
}

// reviewSignal maps a review outcome to a stability update signal ∈ [−1, +1].
//
//	Correct,   accuracy 1.0 → +1.0  (maximum growth)
//	Incorrect, accuracy 0.0 → −1.0  (maximum decay)
func reviewSignal(p reviewPoint) float64 {
	if p.correct {
		return p.accuracy
	}
	return p.accuracy - 1.0
}
