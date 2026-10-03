package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestEvaluateAllLabelsRuleEvalError(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		expression string
		wantSubstr string
	}{
		{"unguarded meta map access", "meta.country == 'US'", "no such key"},
		{"unguarded enrichment map access", "enrichment.ml_score > 0.9", "no such key"},
		{"division by zero", "1 / 0 == 0", "division by zero"},
		{"bad timestamp conversion", `timestamp("not-a-timestamp") != timestamp("2026-01-01T00:00:00Z")`, "type conversion error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, _ := NewEngine(nil, 5)
			defer func() { _ = engine.Close() }()

			if err := engine.LoadRule(&domain.RuleConfig{
				ID: "broken", Name: "n", Expression: tt.expression, Weight: 1.0, Enabled: true,
			}); err != nil {
				t.Fatalf("rule should compile (errors only at eval), but load failed: %v", err)
			}

			res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x"})
			if err != nil {
				t.Fatalf("a per-rule eval error must not fail EvaluateAll, got: %v", err)
			}
			if len(res) != 1 || res[0].SubRuleRef != domain.RuleOutcomeError {
				t.Fatalf("expected one .err result, got %+v", res)
			}
			if !strings.Contains(res[0].Reason, tt.wantSubstr) {
				t.Errorf("expected reason to contain %q, got: %s", tt.wantSubstr, res[0].Reason)
			}
		})
	}
}

// TestEvaluateAllKeepsHealthyRulesWhenOneErrors checks one broken rule does not affect the others.
func TestEvaluateAllKeepsHealthyRulesWhenOneErrors(t *testing.T) {
	ctx := context.Background()
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	for _, r := range []struct{ id, expr string }{
		{"broken", "meta.country == 'US'"},
		{"healthy", "amount > 0.0 ? 1.0 : 0.0"},
	} {
		if err := engine.LoadRule(&domain.RuleConfig{ID: r.id, Name: "n", Expression: r.expr, Weight: 1.0, Enabled: true}); err != nil {
			t.Fatalf("load %s: %v", r.id, err)
		}
	}

	res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x", Amount: 100.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := map[string]domain.RuleResult{}
	for _, r := range res {
		got[r.RuleID] = r
	}
	if got["broken"].SubRuleRef != domain.RuleOutcomeError {
		t.Errorf("broken rule must be labeled .err, got %s", got["broken"].SubRuleRef)
	}
	if h := got["healthy"]; h.SubRuleRef == domain.RuleOutcomeError || h.Score != 1.0 {
		t.Errorf("healthy rule must still score 1.0, got %+v", h)
	}
}

func TestEvaluateAllNoErrorWhenAllRulesClean(t *testing.T) {
	ctx := context.Background()
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "guarded", Name: "g", Expression: "has(meta.country) && meta.country == 'US'", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("load guarded rule: %v", err)
	}

	res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x", Amount: 100.0})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	for _, r := range res {
		if r.SubRuleRef == domain.RuleOutcomeError {
			t.Errorf("rule %s unexpectedly errored: %s", r.RuleID, r.Reason)
		}
	}
}
