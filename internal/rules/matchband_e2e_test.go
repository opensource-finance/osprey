package rules

import (
	"context"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/tadp"
)

func TestMatchBandMissedAlertEndToEnd(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	zero, half, one := 0.0, 0.5, 1.0

	// Rule A: high-amount detection. Score 1.0 for amount > 1000.
	// Bands: [0, 0.5) -> .pass, [0.5, 1.0] -> .fail   (closed last band)
	// Weight 0.3 so the raw-score contribution alone cannot push the
	// aggregate above the default AlertThreshold of 0.7.
	ruleA := &domain.RuleConfig{
		ID:         "high-amount-fail-test",
		Name:       "High Amount Fail Test",
		Expression: "amount > 1000.0 ? 1.0 : 0.0",
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
			{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "High amount fraud"},
		},
		Weight:  0.3,
		Enabled: true,
	}
	if err := engine.LoadRule(ruleA); err != nil {
		t.Fatalf("failed to load ruleA: %v", err)
	}

	// Rule B: always-pass baseline rule with weight 0.7 to dilute the aggregate.
	// Expression "false" -> toScore returns 0.0.  Open-ended band covers everything.
	ruleB := &domain.RuleConfig{
		ID:         "baseline-rule-test",
		Name:       "Baseline Rule Test",
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

	// Evaluate both rules
	results, _ := engine.EvaluateAll(ctx, input)

	// Verify Rule A score is 1.0 but SubRuleRef is wrong
	for _, r := range results {
		if r.RuleID == "high-amount-fail-test" {
			t.Logf("RuleA: Score=%.2f SubRuleRef=%s Reason=%q", r.Score, r.SubRuleRef, r.Reason)
			if r.SubRuleRef == domain.RuleOutcomePass {
				t.Errorf("BUG: RuleA score=1.0 returned .pass (expected .fail from last band [0.5, 1.0])")
			}
		}
	}

	// Pass through TADP processor (detection mode, default AlertThreshold=0.7)
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
		t.Errorf("MISSED ALERT: Status=%s (expected %s). HasCriticalFailure was lost because matchBand "+
			"returned .pass instead of .fail for the last band. Aggregate %.4f < threshold 0.7, "+
			"and no .fail was recorded to force the alert.",
			decision.Status, domain.StatusAlert, decision.Score)
	}
}
