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
	mu             sync.RWMutex
	env            *cel.Env
	compiledRules  map[string]*CompiledRule
	velocityGetter VelocityGetter
	maxWorkers     int
}

// CompiledRule holds a pre-compiled CEL program.
type CompiledRule struct {
	Config  *domain.RuleConfig
	Program cel.Program
}

// VelocityGetter is a function that returns the transaction count for an entity in a time window.
type VelocityGetter func(ctx context.Context, tenantID, entityID string, windowSecs int) (int64, error)

// NewEngine creates a new rule evaluation engine.
func NewEngine(velocityGetter VelocityGetter, maxWorkers int) (*Engine, error) {
	if maxWorkers <= 0 {
		maxWorkers = 10
	}

	// Create CEL environment with transaction variables
	env, err := cel.NewEnv(
		cel.Variable("tx", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("velocity_count", cel.IntType),
		cel.Variable("amount", cel.DoubleType),
		cel.Variable("currency", cel.StringType),
		cel.Variable("debtor_id", cel.StringType),
		cel.Variable("creditor_id", cel.StringType),
		cel.Variable("tx_type", cel.StringType),
		// Balance variables for account drain detection (PaySim pattern)
		cel.Variable("old_balance", cel.DoubleType),
		cel.Variable("new_balance", cel.DoubleType),
	)
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
	Amount         float64
	Currency       string
	VelocityWindow int // seconds
	AdditionalData map[string]any
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

	// Get velocity count if getter is available
	var velocityCount int64
	if e.velocityGetter != nil && input.VelocityWindow > 0 {
		count, err := e.velocityGetter(ctx, input.TenantID, input.DebtorID, input.VelocityWindow)
		if err == nil {
			velocityCount = count
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
		"velocity_count": velocityCount,
		"amount":         input.Amount,
		"currency":       input.Currency,
		"debtor_id":      input.DebtorID,
		"creditor_id":    input.CreditorID,
		"tx_type":        input.Type,
		// Balance variables for account drain detection (default to 0 if not provided)
		"old_balance": 0.0,
		"new_balance": 0.0,
	}

	// Merge additional data
	for k, v := range input.AdditionalData {
		activation[k] = v
	}

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

	// Evaluate CEL expression
	out, _, err := rule.Program.Eval(activation)
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
// except when upper is nil (meaning infinity).
func matchBand(score float64, bands []domain.RuleBand) (string, string) {
	for _, band := range bands {
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
			// Special case: if score equals upper and this is the last band, match it
			if score == upper && band.UpperLimit != nil {
				// Continue to next band which should have this as its lower
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

	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create program for rule %s: %w", cfg.ID, err)
	}

	return &CompiledRule{
		Config:  cfg,
		Program: program,
	}, nil
}
