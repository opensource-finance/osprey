package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

// TestEvaluateAllPropagatesRuleEvalError asserts that a rule which compiles but
// errors at evaluation time causes EvaluateAll to return a non-nil Go error,
// rather than silently returning a zero-score .err result that downstream TADP
// aggregation treats as a clean no-signal outcome (the fail-open bug). The
// error must identify the failing rule and surface the CEL cause so an
// operator can see which rule broke and why.
func TestEvaluateAllPropagatesRuleEvalError(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		ruleID     string
		expression string
		input      *EvaluateInput
		wantSubstr string
	}{
		{
			name:       "unguarded meta map access",
			ruleID:     "meta-unguarded",
			expression: "meta.country == 'US'",
			input:      &EvaluateInput{TenantID: "t", TxID: "x"}, // no AdditionalData -> missing key
			wantSubstr: "no such key",
		},
		{
			name:       "unguarded enrichment map access",
			ruleID:     "enrich-unguarded",
			expression: "enrichment.ml_score > 0.9",
			input:      &EvaluateInput{TenantID: "t", TxID: "x"}, // no Enrichment -> missing key
			wantSubstr: "no such key",
		},
		{
			name:       "division by zero",
			ruleID:     "div-zero",
			expression: "1 / 0 == 0",
			input:      &EvaluateInput{TenantID: "t", TxID: "x"},
			wantSubstr: "division by zero",
		},
		{
			name:       "bad timestamp conversion",
			ruleID:     "bad-timestamp",
			expression: `timestamp("not-a-timestamp") != timestamp("2026-01-01T00:00:00Z")`,
			input:      &EvaluateInput{TenantID: "t", TxID: "x"},
			wantSubstr: "type conversion error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, _ := NewEngine(nil, 5)
			defer func() { _ = engine.Close() }()

			if err := engine.LoadRule(&domain.RuleConfig{
				ID: tt.ruleID, Name: "n", Expression: tt.expression, Weight: 1.0, Enabled: true,
			}); err != nil {
				t.Fatalf("rule should compile (errors only at eval), but load failed: %v", err)
			}

			res, err := engine.EvaluateAll(ctx, tt.input)
			if err == nil {
				t.Fatal("expected non-nil error from EvaluateAll for a rule that errors at eval, got nil")
			}
			if !strings.Contains(err.Error(), tt.ruleID) {
				t.Errorf("expected error to identify failing rule %q, got: %v", tt.ruleID, err)
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Errorf("expected error to contain %q, got: %v", tt.wantSubstr, err)
			}
			if len(res) != 1 {
				t.Fatalf("expected 1 result returned alongside the error, got %d", len(res))
			}
			if res[0].SubRuleRef != domain.RuleOutcomeError {
				t.Errorf("expected .err SubRuleRef, got %s", res[0].SubRuleRef)
			}
			if res[0].Score != 0.0 {
				t.Errorf("expected Score 0 on errored rule, got %.2f", res[0].Score)
			}
			if res[0].Reason == "" {
				t.Error("expected non-empty reason on errored rule result")
			}
		})
	}
}

// TestEvaluateAllPropagatesErrorWithMixedRules asserts that when one rule
// errors and another evaluates cleanly, EvaluateAll still returns a non-nil
// error (so the live decision path fails loudly), while both results are
// returned for inspection. The clean rule must not be lost.
func TestEvaluateAllPropagatesErrorWithMixedRules(t *testing.T) {
	ctx := context.Background()
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "broken", Name: "b", Expression: "meta.country == 'US'", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("load broken rule: %v", err)
	}
	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "healthy", Name: "h", Expression: "amount > 0.0 ? 1.0 : 0.0", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("load healthy rule: %v", err)
	}

	res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x", Amount: 100.0})
	if err == nil {
		t.Fatal("expected non-nil error because one rule errored at eval, got nil")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("expected error to name the broken rule, got: %v", err)
	}
	if !strings.Contains(err.Error(), "1 rule") {
		t.Errorf("expected error to report count of failed rules, got: %v", err)
	}

	if len(res) != 2 {
		t.Fatalf("expected 2 results returned alongside the error, got %d", len(res))
	}
	// Locate results by ID (parallel eval -> non-deterministic slice order).
	var healthy, broken *domain.RuleResult
	for i := range res {
		switch res[i].RuleID {
		case "healthy":
			healthy = &res[i]
		case "broken":
			broken = &res[i]
		}
	}
	if healthy == nil || broken == nil {
		t.Fatalf("expected both healthy and broken results, got %+v", res)
	}
	if healthy.SubRuleRef == domain.RuleOutcomeError {
		t.Errorf("healthy rule must not be labeled .err, got %s (reason: %s)", healthy.SubRuleRef, healthy.Reason)
	}
	if broken.SubRuleRef != domain.RuleOutcomeError {
		t.Errorf("broken rule must be labeled .err, got %s", broken.SubRuleRef)
	}
}

// TestEvaluateAllPropagatesMultipleRuleErrors asserts the error surfaces every
// failing rule, not just the first.
func TestEvaluateAllPropagatesMultipleRuleErrors(t *testing.T) {
	ctx := context.Background()
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	for _, r := range []struct {
		id, expr string
	}{
		{"broken-a", "meta.a == 'x'"},
		{"broken-b", "enrichment.b > 5.0"},
		{"broken-c", "1 / 0 == 0"},
	} {
		if err := engine.LoadRule(&domain.RuleConfig{
			ID: r.id, Name: "n", Expression: r.expr, Weight: 1.0, Enabled: true,
		}); err != nil {
			t.Fatalf("load %s: %v", r.id, err)
		}
	}

	res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x"})
	if err == nil {
		t.Fatal("expected non-nil error with multiple broken rules, got nil")
	}
	for _, id := range []string{"broken-a", "broken-b", "broken-c"} {
		if !strings.Contains(err.Error(), id) {
			t.Errorf("expected error to mention rule %q, got: %v", id, err)
		}
	}
	if !strings.Contains(err.Error(), "3 rule") {
		t.Errorf("expected error to report 3 failed rules, got: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("expected 3 results returned alongside the error, got %d", len(res))
	}
	errCount := 0
	for _, r := range res {
		if r.SubRuleRef == domain.RuleOutcomeError {
			errCount++
		}
	}
	if errCount != 3 {
		t.Errorf("expected all 3 results labeled .err, got %d", errCount)
	}
}

// TestEvaluateAllNoErrorWhenAllRulesClean is the regression guard: rules that
// evaluate cleanly (including guarded-absent meta fields) must NOT produce an
// error. This locks in that the fail-secure propagation only fires on actual
// evaluation failures and does not regress the happy path.
func TestEvaluateAllNoErrorWhenAllRulesClean(t *testing.T) {
	ctx := context.Background()
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "guarded", Name: "g", Expression: "has(meta.country) && meta.country == 'US'", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("load guarded rule: %v", err)
	}
	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "plain", Name: "p", Expression: "amount > 0.0 ? 1.0 : 0.0", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("load plain rule: %v", err)
	}

	// Absent meta.country is guarded -> no eval error -> no propagated error.
	res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x", Amount: 100.0})
	if err != nil {
		t.Fatalf("expected no error when all rules evaluate cleanly, got: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	for _, r := range res {
		if r.SubRuleRef == domain.RuleOutcomeError {
			t.Errorf("rule %s unexpectedly errored: %s", r.RuleID, r.Reason)
		}
	}
}
