package rules

import (
	"log/slog"
	"sync"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// TypologyEngine evaluates typologies based on rule results.
// It calculates weighted scores from individual rule results.
type TypologyEngine struct {
	mu         sync.RWMutex
	typologies map[string]*domain.Typology // key: typologyID
}

// NewTypologyEngine creates a new typology evaluation engine.
func NewTypologyEngine() *TypologyEngine {
	return &TypologyEngine{
		typologies: make(map[string]*domain.Typology),
	}
}

// LoadTypologies loads typology configurations into the engine.
func (e *TypologyEngine) LoadTypologies(typologies []*domain.Typology) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.typologies = make(map[string]*domain.Typology)
	for _, t := range typologies {
		if t.Enabled {
			warnDuplicateTypologyRules(t)
			e.typologies[t.ID] = t
		}
	}
}

// warnDuplicateTypologyRules logs one warning per ruleId that a typology lists
// more than once. The API rejects duplicates, but file-loaded or legacy
// typologies can still reach the engine. evaluateTypology counts only the
// first entry; this warning makes the misconfiguration visible at load time.
func warnDuplicateTypologyRules(t *domain.Typology) {
	seen := make(map[string]bool, len(t.Rules))
	for _, rw := range t.Rules {
		if seen[rw.RuleID] {
			slog.Warn("typology has a duplicate ruleId; later entries are ignored at evaluation",
				"typology_id", t.ID,
				"rule_id", rw.RuleID,
			)
			continue
		}
		seen[rw.RuleID] = true
	}
}

// ReloadTypologies clears and reloads typologies (hot reload).
func (e *TypologyEngine) ReloadTypologies(typologies []*domain.Typology) {
	e.LoadTypologies(typologies)
}

// GetLoadedTypologies returns currently loaded typologies.
func (e *TypologyEngine) GetLoadedTypologies() []*domain.Typology {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*domain.Typology, 0, len(e.typologies))
	for _, t := range e.typologies {
		result = append(result, t)
	}
	return result
}

// TypologyCount returns the number of loaded typologies.
func (e *TypologyEngine) TypologyCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.typologies)
}

// EvaluateTypologies calculates typology scores from rule results.
// For each typology, it calculates a weighted sum of the rule scores
// and determines if the threshold is exceeded.
//
// Algorithm:
// 1. Build a map of ruleID -> score from rule results
// 2. For each typology, sum (rule_score * weight) for matching rules
// 3. Compare against alert threshold
// 4. Return triggered typologies
func (e *TypologyEngine) EvaluateTypologies(ruleResults []domain.RuleResult) []domain.TypologyResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.typologies) == 0 {
		return nil
	}

	// Build rule score map for O(1) lookups
	ruleScores := make(map[string]float64, len(ruleResults))
	for _, r := range ruleResults {
		ruleScores[r.RuleID] = r.Score
	}

	results := make([]domain.TypologyResult, 0, len(e.typologies))

	for _, typology := range e.typologies {
		tStart := time.Now()
		result := e.evaluateTypology(typology, ruleScores)
		result.ProcessMs = time.Since(tStart).Milliseconds()
		results = append(results, result)
	}

	return results
}

// evaluateTypology calculates the score for a single typology.
func (e *TypologyEngine) evaluateTypology(typology *domain.Typology, ruleScores map[string]float64) domain.TypologyResult {
	result := domain.TypologyResult{
		TypologyID:    typology.ID,
		TypologyName:  typology.Name,
		Threshold:     typology.AlertThreshold,
		Contributions: make([]domain.RuleContribution, 0, len(typology.Rules)),
	}

	var totalScore float64
	seen := make(map[string]bool, len(typology.Rules))

	for _, ruleWeight := range typology.Rules {
		ruleScore, exists := ruleScores[ruleWeight.RuleID]
		if !exists {
			// Rule not evaluated - skip
			continue
		}

		if seen[ruleWeight.RuleID] {
			// A typology may list a rule more than once. The API rejects this,
			// but file-loaded or legacy typologies can still reach the engine.
			// Count only the first entry so the score is not inflated.
			continue
		}
		seen[ruleWeight.RuleID] = true

		contribution := ruleScore * ruleWeight.Weight
		totalScore += contribution

		result.Contributions = append(result.Contributions, domain.RuleContribution{
			RuleID:       ruleWeight.RuleID,
			RuleScore:    ruleScore,
			Weight:       ruleWeight.Weight,
			Contribution: contribution,
		})
	}

	result.Score = domain.RoundScore(totalScore)
	result.Triggered = result.Score >= typology.AlertThreshold

	return result
}

// EvaluateTypology evaluates a single typology by ID.
func (e *TypologyEngine) EvaluateTypology(typologyID string, ruleResults []domain.RuleResult) (*domain.TypologyResult, bool) {
	e.mu.RLock()
	typology, exists := e.typologies[typologyID]
	if !exists {
		e.mu.RUnlock()
		return nil, false
	}

	// Build rule score map while holding lock
	ruleScores := make(map[string]float64, len(ruleResults))
	for _, r := range ruleResults {
		ruleScores[r.RuleID] = r.Score
	}

	// Evaluate while holding lock to prevent data race on typology pointer.
	tStart := time.Now()
	result := e.evaluateTypology(typology, ruleScores)
	result.ProcessMs = time.Since(tStart).Milliseconds()
	e.mu.RUnlock()

	return &result, true
}

// GetTriggeredTypologies returns only typologies that exceeded their threshold.
func (e *TypologyEngine) GetTriggeredTypologies(ruleResults []domain.RuleResult) []domain.TypologyResult {
	all := e.EvaluateTypologies(ruleResults)
	triggered := make([]domain.TypologyResult, 0)
	for _, t := range all {
		if t.Triggered {
			triggered = append(triggered, t)
		}
	}
	return triggered
}
