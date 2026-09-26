package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

// TestExtensibleVariables covers the meta/enrichment bags and velocity aggregates.
func TestExtensibleVariables(t *testing.T) {
	ctx := context.Background()

	// The whole meta/enrichment design rests on this: an undeclared bare identifier
	// must FAIL at compile (cel-go's checker is on), so the runtime metadata merge
	// alone never makes new fields usable — they must go through a declared map.
	t.Run("UndeclaredIdentifierFailsCompile", func(t *testing.T) {
		engine, _ := NewEngine(nil, 5)
		defer func() { _ = engine.Close() }()
		err := engine.ValidateRule(&domain.RuleConfig{
			ID: "undeclared", Name: "u", Expression: "country == 'US'", Weight: 1.0, Enabled: true,
		})
		if err == nil {
			t.Fatal("expected compile error for undeclared identifier 'country'")
		}
	})

	// meta.<key> guarded with has(): true when present, false (NOT an error) when absent.
	t.Run("MetaGuardedAccess", func(t *testing.T) {
		engine, _ := NewEngine(nil, 5)
		defer func() { _ = engine.Close() }()
		_ = engine.LoadRule(&domain.RuleConfig{
			ID: "meta-country", Name: "m", Expression: "has(meta.country) && meta.country == 'US'",
			Weight: 1.0, Enabled: true,
		})

		res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x1", AdditionalData: map[string]any{"country": "US"}})
		if err != nil {
			t.Fatalf("eval (present): %v", err)
		}
		if res[0].Score != 1.0 {
			t.Errorf("present: expected score 1.0, got %.2f", res[0].Score)
		}

		res, err = engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x2"})
		if err != nil {
			t.Fatalf("eval (absent): %v", err)
		}
		if res[0].Score != 0.0 {
			t.Errorf("absent: expected score 0.0, got %.2f", res[0].Score)
		}
		if res[0].SubRuleRef == domain.RuleOutcomeError {
			t.Errorf("absent guarded access must not error, got %s", res[0].SubRuleRef)
		}
	})

	// Unguarded reference to a missing key errors at eval — documents the foot-gun.
	// Fail-secure: the evaluation error must be propagated from EvaluateAll
	// (mirroring the velocity-lookup precedent) so the decision path fails loudly
	// as a 5xx instead of silently degrading to a clean no-signal NALT that hides
	// the broken rule from every operator surface.
	t.Run("UnguardedMissingFieldErrors", func(t *testing.T) {
		engine, _ := NewEngine(nil, 5)
		defer func() { _ = engine.Close() }()
		_ = engine.LoadRule(&domain.RuleConfig{
			ID: "meta-unguarded", Name: "m", Expression: "meta.country == 'US'", Weight: 1.0, Enabled: true,
		})
		res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x3"})
		if err == nil {
			t.Fatal("expected evaluation error for unguarded missing key, got nil")
		}
		if !strings.Contains(err.Error(), "meta-unguarded") {
			t.Errorf("expected error to identify failing rule %q, got: %v", "meta-unguarded", err)
		}
		if !strings.Contains(err.Error(), "no such key") {
			t.Errorf("expected error to surface CEL eval cause, got: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result returned alongside the error, got %d", len(res))
		}
		if res[0].SubRuleRef != domain.RuleOutcomeError {
			t.Errorf("expected RuleOutcomeError for unguarded missing key, got %s", res[0].SubRuleRef)
		}
		if res[0].Reason == "" {
			t.Error("expected non-empty reason on the errored rule result")
		}
	})

	// enrichment.<key> guarded access.
	t.Run("EnrichmentGuardedAccess", func(t *testing.T) {
		engine, _ := NewEngine(nil, 5)
		defer func() { _ = engine.Close() }()
		_ = engine.LoadRule(&domain.RuleConfig{
			ID: "enrich-ml", Name: "e", Expression: "has(enrichment.ml_score) && enrichment.ml_score > 0.9",
			Weight: 1.0, Enabled: true,
		})
		res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x4", Enrichment: map[string]any{"ml_score": 0.92}})
		if err != nil {
			t.Fatalf("eval: %v", err)
		}
		if res[0].Score != 1.0 {
			t.Errorf("expected score 1.0, got %.2f", res[0].Score)
		}
	})

	// Velocity aggregate variables are declared and fed by the AggregatesGetter.
	t.Run("VelocityAggregates", func(t *testing.T) {
		engine, _ := NewEngine(nil, 5)
		defer func() { _ = engine.Close() }()
		engine.SetAggregatesGetter(func(_ context.Context, _, _ string, _ int) (VelocityAggregates, error) {
			return VelocityAggregates{Count: 7, AmountSum: 5000.0, DistinctCreditors: 3}, nil
		})
		_ = engine.LoadRule(&domain.RuleConfig{
			ID: "vel-agg", Name: "v", Expression: "velocity_amount_sum > 1000.0 && velocity_distinct_creditors >= 3",
			Weight: 1.0, Enabled: true,
		})
		res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x5", DebtorID: "d", VelocityWindow: 3600})
		if err != nil {
			t.Fatalf("eval: %v", err)
		}
		if res[0].Score != 1.0 {
			t.Errorf("expected score 1.0, got %.2f", res[0].Score)
		}
	})
}
