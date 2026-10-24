package rules

import (
	"context"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// TestMatchBandFractionalBoundaryRoundingEndToEnd locks in the fix for the
// per-rule band-matching rounding bug. A fractional CEL expression
// (0.1 + 0.7) produces float noise (0.7999999999999999). Without
// domain.RoundScore applied in evaluateRule before matchBand, this noise
// falls just below a band boundary of 0.8 and is silently banded .pass,
// losing the .fail force-alert path. After the fix the score is rounded
// to exactly 0.8 and matches the .fail band.
func TestMatchBandFractionalBoundaryRoundingEndToEnd(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	zero, eightTenths, one := 0.0, 0.8, 1.0

	// Rule A: fractional expression whose paper value (0.8) sits exactly on
	// the boundary between .pass and .fail. Weight 0.3 keeps the raw-score
	// contribution alone below the default AlertThreshold of 0.7, so the
	// alert can only fire via the .fail force-alert path.
	ruleA := &domain.RuleConfig{
		ID:         "fractional-boundary-fail-test",
		Name:       "Fractional Boundary Fail Test",
		Expression: "0.1 + 0.7",
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &eightTenths, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
			{LowerLimit: &eightTenths, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "Boundary fraud"},
		},
		Weight:  0.3,
		Enabled: true,
	}
	if err := engine.LoadRule(ruleA); err != nil {
		t.Fatalf("failed to load ruleA: %v", err)
	}

	// Rule B: always-pass baseline rule with weight 0.7 to dilute the
	// aggregate below the alert threshold so the .fail force-alert path is
	// the only thing that can produce an alert.
	ruleB := &domain.RuleConfig{
		ID:         "baseline-fractional-test",
		Name:       "Baseline Fractional Test",
		Expression: "false",
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, SubRuleRef: domain.RuleOutcomePass, Reason: "Baseline"},
		},
		Weight:  0.7,
		Enabled: true,
	}
	if err := engine.LoadRule(ruleB); err != nil {
		t.Fatalf("failed to load ruleB: %v", err)
	}

	ctx := context.Background()
	input := &EvaluateInput{TenantID: "t1", TxID: "tx1", Amount: 2000.0}

	results, _ := engine.EvaluateAll(ctx, input)

	// Verify Rule A: rounded score is exactly 0.8 and outcome is .fail.
	var ruleAResult *domain.RuleResult
	for i := range results {
		if results[i].RuleID == "fractional-boundary-fail-test" {
			ruleAResult = &results[i]
		}
	}
	if ruleAResult == nil {
		t.Fatalf("RuleA result not found")
	}

	t.Logf("RuleA: Score=%.17f SubRuleRef=%s Reason=%q", ruleAResult.Score, ruleAResult.SubRuleRef, ruleAResult.Reason)

	if ruleAResult.Score != 0.8 {
		t.Errorf("RuleA Score=%.17f, want exactly 0.8 (0.1+0.7 rounded to 9 decimals). "+
			"Raw float noise leaked into the persisted score.", ruleAResult.Score)
	}
	if ruleAResult.SubRuleRef != domain.RuleOutcomeFail {
		t.Errorf("BUG: RuleA score=0.1+0.7 (paper 0.8) returned %s (expected .fail from band "+
			"[0.8, 1.0]). Float noise (0.7999999999999999) missed the 0.8 boundary and matched "+
			"the .pass band, losing the .fail force-alert path.", ruleAResult.SubRuleRef)
	}

	// Pass through TADP processor (detection mode, default AlertThreshold=0.7).
	// The weighted aggregate (0.8*0.3 + 0.0*0.7 = 0.24) is below 0.7, so the
	// alert must come from the .fail force-alert path, not the aggregate.
	processor := &tadp.Processor{
		AlertThreshold:     0.7,
		UseWeightedScoring: true,
		Mode:               "detection",
	}

	decision := processor.Process(ctx, &tadp.DecisionInput{
		TenantID:    "t1",
		TxID:        "tx1",
		RuleResults: results,
		StartTime:   time.Now(),
	})

	t.Logf("Aggregate Score: %.4f  Status: %s", decision.Score, decision.Status)

	if decision.Status == domain.StatusNoAlert {
		t.Errorf("MISSED ALERT: Status=%s (expected %s). The .fail force-alert path was lost "+
			"because matchBand banded the noisy 0.7999999999999999 as .pass. Aggregate %.4f < "+
			"threshold 0.7, and no .fail was recorded to force the alert.",
			decision.Status, domain.StatusAlert, decision.Score)
	}
}

// TestPerRuleScoreRoundingNoOpForBooleanRules ensures the rounding fix is a
// no-op for the common boolean-rule case (exact 0.0/1.0 scores), so the fix
// introduces no regression in existing rule sets.
func TestPerRuleScoreRoundingNoOpForBooleanRules(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	zero, half, one := 0.0, 0.5, 1.0

	rule := &domain.RuleConfig{
		ID:         "boolean-rule-noop-test",
		Name:       "Boolean Rule Noop Test",
		Expression: "amount > 1000.0 ? 1.0 : 0.0",
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
			{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "High amount fraud"},
		},
		Weight:  0.3,
		Enabled: true,
	}
	if err := engine.LoadRule(rule); err != nil {
		t.Fatalf("failed to load rule: %v", err)
	}

	ctx := context.Background()

	for _, tc := range []struct {
		name   string
		amount float64
		want   float64
		ref    string
	}{
		{"amount below threshold -> score 0.0 -> .pass", 500.0, 0.0, domain.RuleOutcomePass},
		{"amount above threshold -> score 1.0 -> .fail", 2000.0, 1.0, domain.RuleOutcomeFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results, _ := engine.EvaluateAll(ctx, &EvaluateInput{
				TenantID: "t1", TxID: "tx1", Amount: tc.amount,
			})
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			r := results[0]
			if r.Score != tc.want {
				t.Errorf("Score=%.17f, want %.1f (boolean rules must be unaffected by rounding)",
					r.Score, tc.want)
			}
			if r.SubRuleRef != tc.ref {
				t.Errorf("SubRuleRef=%s, want %s", r.SubRuleRef, tc.ref)
			}
		})
	}
}
