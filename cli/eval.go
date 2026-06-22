package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var boldRe = regexp.MustCompile(`\*\*(.+?)\*\*`)
var nonWordRe = regexp.MustCompile(`[^\w\s]`)

type evalConfig struct {
	ollamaURL string
	username  string
	password  string
	model     string
	threshold float64
}

func evalConfigFrom(cfg appConfig) evalConfig {
	return evalConfig{
		ollamaURL: cfg.OllamaURL,
		username:  cfg.Username,
		password:  cfg.Password,
		model:     cfg.Model,
		threshold: cfg.Threshold,
	}
}

type evalResult struct {
	keywordsScore float64
	accuracy      float64
	correct       bool
	latencyMs     int64
}

type evaluator struct {
	cfg    evalConfig
	client *http.Client
}

func newEvaluator(cfg evalConfig) *evaluator {
	return &evaluator{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}}
}

func (e *evaluator) evaluate(concept, referenceMD, userAnswer string) (evalResult, error) {
	ks := keywordsScore(referenceMD, userAnswer)

	start := time.Now()
	acc, err := e.llmAccuracy(concept, stripBold(referenceMD), userAnswer)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return evalResult{}, err
	}

	return evalResult{
		keywordsScore: ks,
		accuracy:      acc,
		correct:       ks >= e.cfg.threshold && acc >= e.cfg.threshold,
		latencyMs:     latency,
	}, nil
}

// keywordsScore returns the minimum partial-ratio match across all **bold**
// keywords in the reference. Mirrors Python's fuzzy_keywords_score logic.
func keywordsScore(referenceMD, userAnswer string) float64 {
	matches := boldRe.FindAllStringSubmatch(referenceMD, -1)
	if len(matches) == 0 {
		return 1.0
	}
	normAnswer := normalizeText(userAnswer)
	min := 1.0
	for _, m := range matches {
		s := partialRatio(normalizeText(m[1]), normAnswer)
		if s < min {
			min = s
		}
	}
	return min
}

func stripBold(md string) string {
	return boldRe.ReplaceAllString(md, "$1")
}

func normalizeText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)))
	result, _, _ := transform.String(t, s)
	return nonWordRe.ReplaceAllString(result, " ")
}

// partialRatio returns the best similarity of `short` against any same-length
// window of `long`, matching Python rapidfuzz fuzz.partial_ratio.
func partialRatio(short, long string) float64 {
	rs := []rune(short)
	rl := []rune(long)
	ls, ll := len(rs), len(rl)

	if ls == 0 {
		return 1.0
	}
	if ls >= ll {
		return levenshteinSim(rs, rl)
	}

	best := 0.0
	for i := range ll - ls + 1 {
		if s := levenshteinSim(rs, rl[i:i+ls]); s > best {
			best = s
		}
	}
	return best
}

// levenshteinSim returns 1 - normalised edit distance.
func levenshteinSim(a, b []rune) float64 {
	la, lb := len(a), len(b)
	if la == 0 && lb == 0 {
		return 1.0
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}

	maxLen := max(la, lb)
	return 1.0 - float64(prev[lb])/float64(maxLen)
}

var llmSystemPrompt = "Tu évalues le SENS de la réponse d'un étudiant à une flashcard, " +
	"comparée à une réponse de référence. Réponds par 'accuracy', un " +
	"score entre 0 et 1 : 1 = sens complet et exact, 0.5 = partiel ou " +
	"imprécis, 0 = faux ou hors-sujet. Tolère les fautes d'orthographe, " +
	"ignore le style et la longueur."

var llmJSONSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"accuracy": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
	},
	"required": []string{"accuracy"},
}

func (e *evaluator) llmAccuracy(concept, referencePlain, userAnswer string) (float64, error) {
	payload := map[string]any{
		"model": e.cfg.model,
		"messages": []map[string]string{
			{"role": "system", "content": llmSystemPrompt},
			{"role": "user", "content": fmt.Sprintf(
				"Concept: %s\nRéférence: %s\nRéponse étudiant: %s",
				concept, referencePlain, userAnswer,
			)},
		},
		"format":  llmJSONSchema,
		"options": map[string]any{"num_ctx": 512, "temperature": 0.1},
		"stream":  false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", e.cfg.ollamaURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(e.cfg.username, e.cfg.password)

	resp, err := e.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("ollama status %d", resp.StatusCode)
	}

	var apiResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	var result struct {
		Accuracy float64 `json:"accuracy"`
	}
	if err := json.Unmarshal([]byte(apiResp.Message.Content), &result); err != nil {
		return 0, fmt.Errorf("decode accuracy: %w", err)
	}

	return max(0.0, min(1.0, result.Accuracy)), nil
}
