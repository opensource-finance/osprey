package rules

import (
	"context"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestSanctionsScoreTakesWorsePartyAndFailsRule(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	zero, hit := 0.0, 0.92
	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "sanctions-001", Expression: "sanctions_match != '' ? sanctions_score : 0.0", Weight: 1, Enabled: true,
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &hit, SubRuleRef: domain.RuleOutcomePass},
			{LowerLimit: &hit, SubRuleRef: domain.RuleOutcomeFail, Reason: "SDN hit"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	input := &EvaluateInput{TenantID: "t", TxID: "tx", Amount: 1, DebtorName: "Alice", CreditorName: "Bad Actor"}

	// No screener wired: screening is off, rule passes.
	res, err := engine.EvaluateAll(ctx, input)
	if err != nil || res[0].SubRuleRef != domain.RuleOutcomePass || res[0].Score != 0 {
		t.Fatalf("no screener: %+v %v", res, err)
	}

	calls := 0
	engine.SetSanctionsScreener(func(name string) (float64, string) {
		calls++
		if name == "Bad Actor" {
			return 0.97, "ACTOR, Bad"
		}
		return 0.4, "SOMEONE"
	})
	res, err = engine.EvaluateAll(ctx, input)
	if err != nil || res[0].SubRuleRef != domain.RuleOutcomeFail || res[0].Score != 0.97 {
		t.Fatalf("creditor hit: %+v %v", res, err)
	}
	if res[0].Reason != "SDN hit: ACTOR, Bad" {
		t.Errorf("reason = %q, want the matched entry appended", res[0].Reason)
	}

	// No loaded rule reads sanctions variables: the screener is not called.
	if err := engine.ReloadRules([]*domain.RuleConfig{{ID: "amount", Expression: "amount > 0.0", Weight: 1, Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	calls = 0
	if _, err := engine.EvaluateAll(ctx, input); err != nil || calls != 0 {
		t.Errorf("screener called %d times with no sanctions rule (err %v)", calls, err)
	}
}
