package rules

import (
	"sync"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// TypologyEngine evaluates typologies based on rule results.
// It calculates weighted scores from individual rule results.
type TypologyEngine struct {
	mu         sync.RWMutex
	typologies map[string]*domain.Typology // key: typologyID
	// generation is bumped on every wholesale typology replacement
	// (LoadTypologies/ReloadTypologies). Compliance-mode callers capture it
	// at request entry and re-check it before deciding, so a concurrent admin
	// operation that empties or replaces the typology set mid-request is
	// detected and rejected rather than silently producing a detection-mode
	// decision. See internal/api/handler.go and internal/worker/worker.go.
	generation uint64
}

// NewTypologyEngine creates a new typology evaluation engine.
func NewTypologyEngine() *TypologyEngine {
	return &TypologyEngine{
		typologies: make(map[string]*domain.Typology),
	}
}

// LoadTypologies loads typology configurations into the engine.
// Each call is an atomic wholesale replacement and bumps the generation
// counter so in-flight compliance-mode evaluations can detect the change.
func (e *TypologyEngine) LoadTypologies(typologies []*domain.Typology) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.typologies = make(map[string]*domain.Typology)
	for _, t := range typologies {
		if t.Enabled {
			e.typologies[t.ID] = t
		}
	}
	e.generation++
}

// ReloadTypologies clears and reloads typologies (hot reload).
func (e *TypologyEngine) ReloadTypologies(typologies []*domain.Typology) {
	e.LoadTypologies(typologies)
}

// Generation returns the current typology-set generation. It is bumped on
// every LoadTypologies/ReloadTypologies call. Callers capture it atomically
// with the typology count via Snapshot to close the TOCTOU window between a
// compliance-mode entry guard and the later typology evaluation.
func (e *TypologyEngine) Generation() uint64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.generation
}

// Snapshot returns the current generation and typology count atomically
// (under a single lock). Compliance-mode callers capture a Snapshot at
// request entry: count == 0 means "reject now" (the entry guard), and
// generation is later compared by EvaluateTypologiesIfStable to detect a
// concurrent reload that emptied or replaced the set mid-request.
func (e *TypologyEngine) Snapshot() (generation uint64, count int) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.generation, len(e.typologies)
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

	return e.evaluateTypologiesLocked(ruleResults)
}

// EvaluateTypologiesIfStable evaluates typologies only when the typology set
// is unchanged since expectedGen and non-empty. The generation check and the
// evaluation happen under a single lock, so a concurrent LoadTypologies call
// (which needs the write lock) cannot empty or replace the set between the
// check and the evaluation.
//
// Returns (results, true) when the set is stable and non-empty: results is the
// typology evaluation against the same set the caller observed at entry, so the
// decision is consistent with the configuration that admitted the request.
//
// Returns (nil, false) when the set changed since expectedGen or is now empty.
// Compliance-mode callers MUST fail closed (reject the request / skip the
// transaction) when ok is false; proceeding with nil results would silently
// degrade to detection-mode scoring. See internal/tadp/tadp.go.
func (e *TypologyEngine) EvaluateTypologiesIfStable(ruleResults []domain.RuleResult, expectedGen uint64) ([]domain.TypologyResult, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.generation != expectedGen || len(e.typologies) == 0 {
		return nil, false
	}

	return e.evaluateTypologiesLocked(ruleResults), true
}

// evaluateTypologiesLocked computes typology scores while holding the read
// lock. Callers must hold e.mu (RLock or Lock).
func (e *TypologyEngine) evaluateTypologiesLocked(ruleResults []domain.RuleResult) []domain.TypologyResult {
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

	for _, ruleWeight := range typology.Rules {
		ruleScore, exists := ruleScores[ruleWeight.RuleID]
		if !exists {
			// Rule not evaluated - skip
			continue
		}

		contribution := ruleScore * ruleWeight.Weight
		totalScore += contribution

		result.Contributions = append(result.Contributions, domain.RuleContribution{
			RuleID:       ruleWeight.RuleID,
			RuleScore:    ruleScore,
			Weight:       ruleWeight.Weight,
			Contribution: contribution,
		})
	}

	result.Score = totalScore
	result.Triggered = totalScore >= typology.AlertThreshold

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
