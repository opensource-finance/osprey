// Package rules provides the CEL-Go based rule evaluation engine.
package rules

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/opensource-finance/osprey/internal/domain"
)

// Engine is the CEL-based rule evaluation engine.
type Engine struct {
	mu                sync.RWMutex
	env               *cel.Env
	compiledRules     map[string]*CompiledRule
	velocityGetter    VelocityGetter
	aggregatesGetter  AggregatesGetter
	eventClock        bool
	sanctionsScreener SanctionsScreener
	maxWorkers        int
}

// CompiledRule holds a pre-compiled CEL program.
type CompiledRule struct {
	Config  *domain.RuleConfig
	Program cel.Program

	usesSanctions bool // expression reads sanctions_score or sanctions_match
}

// VelocityGetter is a function that returns the transaction count for an entity in a time window.
type VelocityGetter func(ctx context.Context, tenantID, entityID string, windowSecs int) (int64, error)

// VelocityAggregates holds windowed velocity metrics for an entity.
type VelocityAggregates struct {
	Count             int64
	AmountSum         float64
	DistinctCreditors int64
	InboundCount      int64
	InboundAmountSum  float64
}

// AggregatesGetter returns windowed velocity aggregates for an entity. When set
// on the engine it supersedes VelocityGetter and additionally populates the
// velocity_amount_sum, velocity_distinct_creditors and velocity_inbound_* CEL
// variables. A zero at means a window that ends now on the received clock; a
// non-zero at means a window that ends at that event time on the event clock.
type AggregatesGetter func(ctx context.Context, tenantID, entityID string, windowSecs int, at time.Time) (VelocityAggregates, error)

// SanctionsScreener returns the best sanctions-list match for a party name.
// listedEntry identifies the matched list entry for analysts, e.g.
// "SAMPLE, Jonathan Quill (OFAC ent 1002, PROGRAM-A)".
type SanctionsScreener func(name string) (score float64, listedEntry string)

// SetSanctionsScreener wires sanctions screening. Optional: if unset,
// sanctions_score is 0 and sanctions_match is empty.
func (e *Engine) SetSanctionsScreener(s SanctionsScreener) {
	e.sanctionsScreener = s
}

// screen returns the higher-scoring of the debtor and creditor matches.
func (e *Engine) screen(input *EvaluateInput) (float64, string) {
	if e.sanctionsScreener == nil {
		return 0, ""
	}
	var best float64
	var match string
	for _, name := range []string{input.DebtorName, input.CreditorName} {
		if name == "" {
			continue
		}
		if s, m := e.sanctionsScreener(name); s > best {
			best, match = s, m
		}
	}
	return best, match
}

// SetAggregatesGetter wires a richer velocity source. Optional and additive:
// if unset, the engine falls back to the count-only VelocityGetter.
func (e *Engine) SetAggregatesGetter(g AggregatesGetter) {
	e.aggregatesGetter = g
}

// SetEventClock makes velocity windows end at each transaction's event time
// (EvaluateInput.EventTime) instead of now. Off by default: the event time comes
// from the caller, so a backdated timestamp can move the window.
func (e *Engine) SetEventClock(on bool) {
	e.eventClock = on
}

// maxCELCost bounds the runtime cost of a single rule expression. CEL is
// non-Turing-complete and the environment exposes no string-extension or
// unbounded functions, so legitimate rules cost only a handful of units; this
// limit is a defense-in-depth backstop against a pathological expression
// exhausting CPU during evaluation.
const maxCELCost uint64 = 1_000_000

// NewEngine creates a new rule evaluation engine.
func NewEngine(velocityGetter VelocityGetter, maxWorkers int) (*Engine, error) {
	if maxWorkers <= 0 {
		maxWorkers = 10
	}

	// Create CEL environment from the canonical variable Catalog (variables.go).
	env, err := cel.NewEnv(EnvOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &Engine{
		env:            env,
		compiledRules:  make(map[string]*CompiledRule),
		velocityGetter: velocityGetter,
		maxWorkers:     maxWorkers,
	}, nil
}

// ValidateRule compiles and validates a rule without mutating loaded engine rules.
func (e *Engine) ValidateRule(cfg *domain.RuleConfig) error {
	if cfg == nil {
		return fmt.Errorf("rule config is required")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	_, err := e.compileRule(cfg)
	return err
}

// LoadRule compiles and loads a rule into the engine.
func (e *Engine) LoadRule(cfg *domain.RuleConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	compiled, err := e.compileRule(cfg)
	if err != nil {
		return err
	}

	e.compiledRules[cfg.ID] = compiled

	return nil
}

// LoadRules compiles and loads multiple rules.
func (e *Engine) LoadRules(configs []*domain.RuleConfig) error {
	for _, cfg := range configs {
		if cfg.Enabled {
			if err := e.LoadRule(cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

// EvaluateInput holds the transaction data for rule evaluation.
type EvaluateInput struct {
	TenantID       string
	TxID           string
	Type           string
	DebtorID       string
	CreditorID     string
	DebtorName     string // optional; screened against the sanctions list
	CreditorName   string // optional; screened against the sanctions list
	Amount         float64
	Currency       string
	VelocityWindow int            // seconds
	EventTime      time.Time      // transaction timestamp; ends the velocity window when the event clock is on
	AdditionalData map[string]any // request metadata -> CEL `meta` map
	Enrichment     map[string]any // externally-computed scores -> CEL `enrichment` map
}

// EvaluateAll evaluates all loaded rules in parallel.
func (e *Engine) EvaluateAll(ctx context.Context, input *EvaluateInput) ([]domain.RuleResult, error) {
	e.mu.RLock()
	rules := make([]*CompiledRule, 0, len(e.compiledRules))
	for _, rule := range e.compiledRules {
		rules = append(rules, rule)
	}
	e.mu.RUnlock()

	if len(rules) == 0 {
		return nil, nil
	}

	// Compute velocity aggregates if a getter is available. Prefer the richer
	// AggregatesGetter (count + amount-sum + distinct-counterparties); fall back
	// to the count-only VelocityGetter.
	//
	// Fail-secure: a velocity lookup failure must NOT silently become zero, which
	// would let velocity rules (e.g. velocity_count > 5) score 0 and pass a
	// high-velocity transaction. Surface the error so the evaluation fails loudly
	// (caller logs it + returns 5xx) rather than emitting a false-negative decision.
	var agg VelocityAggregates
	if input.VelocityWindow > 0 {
		switch {
		case e.aggregatesGetter != nil:
			var at time.Time
			if e.eventClock {
				at = input.EventTime
			}
			var err error
			agg, err = e.aggregatesGetter(ctx, input.TenantID, input.DebtorID, input.VelocityWindow, at)
			if err != nil {
				return nil, fmt.Errorf("velocity lookup failed for entity %q: %w", input.DebtorID, err)
			}
		case e.velocityGetter != nil:
			count, err := e.velocityGetter(ctx, input.TenantID, input.DebtorID, input.VelocityWindow)
			if err != nil {
				return nil, fmt.Errorf("velocity lookup failed for entity %q: %w", input.DebtorID, err)
			}
			agg.Count = count
		}
	}

	// Screen only when a loaded rule reads the result; it is the costliest input.
	var sanctionsScore float64
	var sanctionsMatch string
	for _, r := range rules {
		if r.usesSanctions {
			sanctionsScore, sanctionsMatch = e.screen(input)
			break
		}
	}

	// Prepare CEL activation variables

	activation := map[string]any{
		"tx": map[string]any{
			"id":          input.TxID,
			"type":        input.Type,
			"debtor_id":   input.DebtorID,
			"creditor_id": input.CreditorID,
			"amount":      input.Amount,
			"currency":    input.Currency,
		},
		"velocity_count":              agg.Count,
		"velocity_amount_sum":         agg.AmountSum,
		"velocity_distinct_creditors": agg.DistinctCreditors,
		"velocity_inbound_count":      agg.InboundCount,
		"velocity_inbound_amount_sum": agg.InboundAmountSum,
		"amount":                      input.Amount,
		"currency":                    input.Currency,
		"debtor_id":                   input.DebtorID,
		"creditor_id":                 input.CreditorID,
		"tx_type":                     input.Type,
		"sanctions_score":             sanctionsScore,
		"sanctions_match":             sanctionsMatch,
		// Balance variables for account drain detection (default to 0 if not provided)
		"old_balance": 0.0,
		"new_balance": 0.0,
	}

	// Metadata may only set metadata-sourced vars, never amount, tx_type, velocity_*, etc.
	for k, v := range input.AdditionalData {
		if _, ok := metadataOverridable[k]; !ok {
			continue
		}
		f, ok, err := metadataNumber(k, v)
		if err != nil {
			return nil, err
		}
		if ok {
			activation[k] = f
		}
	}

	// Open-ended bags. Set AFTER the merge so metadata keys can't clobber them.
	// Always present (possibly empty) so has(meta.x)/has(enrichment.x) never error.
	activation["meta"] = mapOrEmpty(input.AdditionalData)
	activation["enrichment"] = mapOrEmpty(input.Enrichment)

	// Parallel evaluation using worker pool pattern
	results := make([]domain.RuleResult, len(rules))
	var wg sync.WaitGroup

	// Limit concurrency with semaphore
	sem := make(chan struct{}, e.maxWorkers)

	for i, rule := range rules {
		wg.Add(1)
		go func(idx int, r *CompiledRule) {
			defer wg.Done()

			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			result := e.evaluateRule(ctx, r, activation, input)
			results[idx] = result
		}(i, rule)
	}

	wg.Wait()

	return results, nil
}

// evaluateRule evaluates a single rule and returns the result.
func (e *Engine) evaluateRule(ctx context.Context, rule *CompiledRule, activation map[string]any, input *EvaluateInput) domain.RuleResult {
	start := time.Now()

	result := domain.RuleResult{
		RuleID:   rule.Config.ID,
		TenantID: input.TenantID,
		TxID:     input.TxID,
		Weight:   rule.Config.Weight,
	}

	// Evaluate CEL expression. ContextEval honors request cancellation and the
	// program's cost limit, so a slow or pathological rule cannot block the
	// evaluation indefinitely.
	out, _, err := rule.Program.ContextEval(ctx, activation)
	if err != nil {
		result.SubRuleRef = domain.RuleOutcomeError
		result.Reason = fmt.Sprintf("evaluation error: %v", err)
		result.ProcessMs = time.Since(start).Milliseconds()
		return result
	}

	// Convert result to score
	score := toScore(out)
	result.Score = score

	// Determine outcome based on bands
	result.SubRuleRef, result.Reason = matchBand(score, rule.Config.Bands)
	// Name the listed entry on a sanctions alert so it can be dispositioned
	// from the stored evaluation alone.
	if m, _ := activation["sanctions_match"].(string); rule.usesSanctions && m != "" && result.SubRuleRef != domain.RuleOutcomePass {
		result.Reason += ": " + m
	}
	result.ProcessMs = time.Since(start).Milliseconds()

	return result
}

// toScore converts a CEL value to a numeric score.
func toScore(val ref.Val) float64 {
	switch v := val.(type) {
	case types.Bool:
		if v {
			return 1.0
		}
		return 0.0
	case types.Double:
		return float64(v)
	case types.Int:
		return float64(v)
	default:
		return 0.0
	}
}

// matchBand finds the matching band for a score.
// Bands are evaluated in order. Use lower inclusive, upper exclusive,
// except when upper is nil (meaning infinity), or when the score equals
// the upper limit of the last band (closed last band), which matches
// inclusively so scores at the boundary do not fall through to the
// default .pass outcome.
func matchBand(score float64, bands []domain.RuleBand) (string, string) {
	for i, band := range bands {
		lower := 0.0
		hasUpper := band.UpperLimit != nil
		upper := float64(1e9) // effectively infinity

		if band.LowerLimit != nil {
			lower = *band.LowerLimit
		}
		if hasUpper {
			upper = *band.UpperLimit
		}

		// Match: lower <= score < upper (or lower <= score if no upper bound)
		if score >= lower {
			if !hasUpper || score < upper {
				return band.SubRuleRef, band.Reason
			}
			// Score equals the upper boundary. A non-last band defers to the
			// next band (which should have this value as its lower). The last
			// band has no successor, so match it inclusively to avoid
			// falling through to the default .pass "no matching band".
			if score == upper {
				if i == len(bands)-1 {
					return band.SubRuleRef, band.Reason
				}
				continue
			}
		}
	}

	// Default to pass if no band matches
	return domain.RuleOutcomePass, "no matching band"
}

// RulesCount returns the number of loaded rules.
func (e *Engine) RulesCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.compiledRules)
}

// ReloadRules clears all existing rules and loads new ones.
// This enables hot-reloading of rules from the database.
func (e *Engine) ReloadRules(configs []*domain.RuleConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	newRules := make(map[string]*CompiledRule)

	// Load new rules
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}

		compiled, err := e.compileRule(cfg)
		if err != nil {
			return err
		}
		newRules[cfg.ID] = compiled
	}

	e.compiledRules = newRules

	return nil
}

// GetLoadedRules returns the currently loaded rule configurations.
func (e *Engine) GetLoadedRules() []*domain.RuleConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*domain.RuleConfig, 0, len(e.compiledRules))
	for _, compiled := range e.compiledRules {
		rules = append(rules, compiled.Config)
	}
	return rules
}

// Close cleans up the engine.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.compiledRules = make(map[string]*CompiledRule)
	return nil
}

func (e *Engine) compileRule(cfg *domain.RuleConfig) (*CompiledRule, error) {
	ast, issues := e.env.Compile(cfg.Expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile rule %s: %w", cfg.ID, issues.Err())
	}

	outputType := ast.OutputType()
	if outputType != cel.BoolType && outputType != cel.DoubleType && outputType != cel.IntType {
		return nil, fmt.Errorf("rule %s: expression must return bool, int, or double, got %s", cfg.ID, outputType)
	}

	program, err := e.env.Program(ast, cel.CostLimit(maxCELCost))
	if err != nil {
		return nil, fmt.Errorf("failed to create program for rule %s: %w", cfg.ID, err)
	}

	usesSanctions := false
	for _, ref := range ast.NativeRep().ReferenceMap() {
		if ref.Name == "sanctions_score" || ref.Name == "sanctions_match" {
			usesSanctions = true
		}
	}

	return &CompiledRule{
		Config:        cfg,
		Program:       program,
		usesSanctions: usesSanctions,
	}, nil
}
