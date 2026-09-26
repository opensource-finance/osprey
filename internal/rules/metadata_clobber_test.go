package rules

import (
	"context"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

// clobberBands is the standard 0/1 band pair used by the metadata-clobber tests:
// score == 0.0 resolves to .pass, score >= 1.0 resolves to .fail.
func clobberBands() []domain.RuleBand {
	zero, one := 0.0, 1.0
	return []domain.RuleBand{
		{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomePass, Reason: "no trigger"},
		{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeFail, Reason: "trigger"},
	}
}

// loadClobberRule compiles a single rule with expression `expr` and standard bands.
func loadClobberRule(t *testing.T, eng *Engine, id, expr string) {
	t.Helper()
	if err := eng.LoadRule(&domain.RuleConfig{
		ID: id, Name: id, Expression: expr, Bands: clobberBands(), Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("load rule %q (%s): %v", id, expr, err)
	}
}

// evalClobber evaluates a single rule against a high-value PAYMENT transaction and
// returns the result. The authoritative fields (amount=500000, tx_type=PAYMENT,
// currency=USD, debtor_id=d-1, creditor_id=c-1) are chosen so that real-value rules
// trigger; metadata is the attacker-controlled payload under test.
func evalClobber(t *testing.T, eng *Engine, metadata map[string]any) domain.RuleResult {
	t.Helper()
	ctx := context.Background()
	input := &EvaluateInput{
		TenantID:       "tenant-1",
		TxID:           "tx-1",
		Type:           "PAYMENT",
		DebtorID:       "d-1",
		CreditorID:     "c-1",
		Amount:         500000.0,
		Currency:       "USD",
		VelocityWindow: 0, // no velocity lookup; velocity_count stays 0
		AdditionalData: metadata,
	}
	results, err := eng.EvaluateAll(ctx, input)
	if err != nil {
		t.Fatalf("EvaluateAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	return results[0]
}

// TestMetadataClobberRepro asserts that caller-supplied metadata keys whose names
// collide with engine-authoritative Catalog variables can no longer override the
// authoritative value in the CEL activation. Each case mirrors a row of the bug
// reproduction table: before the fix every colliding key clobbered the real value
// (bypassing or forcing the rule); after the fix the real value always wins.
//
// Pre-fix behaviour documented per case (in comments) for regression traceability.
func TestMetadataClobberRepro(t *testing.T) {
	ctx := context.Background()
	_ = ctx

	type expect struct {
		score    float64
		outcome  string
		hasError bool // result.SubRuleRef == RuleOutcomeError
	}

	cases := []struct {
		name     string
		metadata map[string]any
		ruleID   string
		expr     string
		// expected AFTER the fix — the engine-authoritative value wins.
		want expect
		// preFix documents the observed behaviour before the fix (for context only).
		preFix expect
	}{
		{
			name:     "amount clobber attempt bypassed",
			metadata: map[string]any{"amount": 1.0},
			ruleID:   "amt",
			expr:     "amount > 200000.0 ? 1.0 : 0.0",
			want:     expect{score: 1.0, outcome: domain.RuleOutcomeFail},
			preFix:   expect{score: 0.0, outcome: domain.RuleOutcomePass},
		},
		{
			name:     "tx_type clobber attempt forcing fire prevented",
			metadata: map[string]any{"tx_type": "CASH_OUT"},
			ruleID:   "txt",
			expr:     `tx_type == "CASH_OUT" ? 1.0 : 0.0`,
			want:     expect{score: 0.0, outcome: domain.RuleOutcomePass},
			preFix:   expect{score: 1.0, outcome: domain.RuleOutcomeFail},
		},
		{
			name:     "tx map clobber attempt bypassed",
			metadata: map[string]any{"tx": map[string]any{"amount": 1.0}},
			ruleID:   "txmap",
			expr:     "tx.amount > 200000.0 ? 1.0 : 0.0",
			want:     expect{score: 1.0, outcome: domain.RuleOutcomeFail},
			preFix:   expect{score: 0.0, outcome: domain.RuleOutcomePass},
		},
		{
			name:     "wrong-typed amount shadow no longer errors (real value wins)",
			metadata: map[string]any{"amount": "not-a-number"},
			ruleID:   "amttype",
			expr:     "amount > 200000.0 ? 1.0 : 0.0",
			want:     expect{score: 1.0, outcome: domain.RuleOutcomeFail},
			preFix:   expect{score: 0.0, outcome: domain.RuleOutcomeError, hasError: true},
		},
		{
			name:     "velocity_count clobber attempt forcing fire prevented",
			metadata: map[string]any{"velocity_count": 50.0},
			ruleID:   "velcount",
			expr:     "velocity_count > 10 ? 1.0 : 0.0",
			want:     expect{score: 0.0, outcome: domain.RuleOutcomePass},
			preFix:   expect{score: 1.0, outcome: domain.RuleOutcomeFail},
		},
		{
			name:     "velocity_amount_sum clobber attempt forcing fire prevented",
			metadata: map[string]any{"velocity_amount_sum": 5000.0},
			ruleID:   "velsum",
			expr:     "velocity_amount_sum > 1000.0 ? 1.0 : 0.0",
			want:     expect{score: 0.0, outcome: domain.RuleOutcomePass},
			preFix:   expect{score: 1.0, outcome: domain.RuleOutcomeFail},
		},
		{
			name:     "velocity_distinct_creditors clobber attempt bypassed",
			metadata: map[string]any{"velocity_distinct_creditors": 100.0},
			ruleID:   "veldistinct",
			expr:     "velocity_distinct_creditors > 3 ? 1.0 : 0.0",
			want:     expect{score: 0.0, outcome: domain.RuleOutcomePass},
			preFix:   expect{score: 1.0, outcome: domain.RuleOutcomeFail},
		},
		{
			name:     "currency clobber attempt prevented",
			metadata: map[string]any{"currency": "EUR"},
			ruleID:   "cur",
			expr:     `currency == "USD" ? 1.0 : 0.0`,
			want:     expect{score: 1.0, outcome: domain.RuleOutcomeFail},
			preFix:   expect{score: 0.0, outcome: domain.RuleOutcomePass},
		},
		{
			name:     "debtor_id clobber attempt prevented",
			metadata: map[string]any{"debtor_id": "attacker"},
			ruleID:   "deb",
			expr:     `debtor_id == "d-1" ? 1.0 : 0.0`,
			want:     expect{score: 1.0, outcome: domain.RuleOutcomeFail},
			preFix:   expect{score: 0.0, outcome: domain.RuleOutcomePass},
		},
		{
			name:     "creditor_id clobber attempt prevented",
			metadata: map[string]any{"creditor_id": "attacker"},
			ruleID:   "cred",
			expr:     `creditor_id == "c-1" ? 1.0 : 0.0`,
			want:     expect{score: 1.0, outcome: domain.RuleOutcomeFail},
			preFix:   expect{score: 0.0, outcome: domain.RuleOutcomePass},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng, _ := NewEngine(nil, 5)
			defer func() { _ = eng.Close() }()
			loadClobberRule(t, eng, tc.ruleID, tc.expr)

			got := evalClobber(t, eng, tc.metadata)

			if got.Score != tc.want.score {
				t.Errorf("score: got %.2f, want %.2f (pre-fix was %.2f)", got.Score, tc.want.score, tc.preFix.score)
			}
			if got.SubRuleRef != tc.want.outcome {
				t.Errorf("outcome: got %s, want %s (pre-fix was %s)", got.SubRuleRef, tc.want.outcome, tc.preFix.outcome)
			}
			if got.SubRuleRef == domain.RuleOutcomeError != tc.want.hasError {
				t.Errorf("error flag: got error=%v, want %v", got.SubRuleRef == domain.RuleOutcomeError, tc.want.hasError)
			}
		})
	}

	// Sanity: the velocity cases above use a nil getter (velocity_count/sum = 0).
	// Confirm the velocity aggregates still read the engine-computed value when a
	// real AggregatesGetter is wired, and that a colliding metadata key cannot
	// override it.
	t.Run("velocity aggregates from getter win over metadata", func(t *testing.T) {
		eng, _ := NewEngine(nil, 5)
		defer func() { _ = eng.Close() }()
		eng.SetAggregatesGetter(func(_ context.Context, _, _ string, _ int) (VelocityAggregates, error) {
			return VelocityAggregates{Count: 15, AmountSum: 5000.0, DistinctCreditors: 4}, nil
		})
		loadClobberRule(t, eng, "vel-from-getter", "velocity_amount_sum > 1000.0 && velocity_distinct_creditors >= 3 ? 1.0 : 0.0")

		ctx := context.Background()
		input := &EvaluateInput{
			TenantID:       "t",
			TxID:           "x",
			DebtorID:       "d",
			VelocityWindow: 3600,
			// Attacker tries to zero out the aggregates.
			AdditionalData: map[string]any{
				"velocity_count":              0.0,
				"velocity_amount_sum":         0.0,
				"velocity_distinct_creditors": 0.0,
			},
		}
		results, err := eng.EvaluateAll(ctx, input)
		if err != nil {
			t.Fatalf("EvaluateAll: %v", err)
		}
		if results[0].Score != 1.0 {
			t.Errorf("expected score 1.0 (getter values win), got %.2f", results[0].Score)
		}
		if results[0].SubRuleRef != domain.RuleOutcomeFail {
			t.Errorf("expected .fail, got %s", results[0].SubRuleRef)
		}
	})
}

// TestMetadataClobberBaseline asserts the happy path is unchanged by the fix:
// non-colliding metadata (e.g. country, mcc) is still accessible via the meta bag,
// and engine-authoritative rules still trigger on the real values when no key
// collides. This guards against an over-broad fix that would break legitimate
// metadata use.
func TestMetadataClobberBaseline(t *testing.T) {
	t.Run("non-colliding metadata keeps real amount and exposes meta bag", func(t *testing.T) {
		eng, _ := NewEngine(nil, 5)
		defer func() { _ = eng.Close() }()
		loadClobberRule(t, eng, "high-value", "amount > 200000.0 ? 1.0 : 0.0")

		// metadata with a key that does NOT collide with any Catalog variable.
		got := evalClobber(t, eng, map[string]any{"country": "US", "mcc": "5411"})
		if got.Score != 1.0 {
			t.Errorf("expected score 1.0 for real high-value tx, got %.2f", got.Score)
		}
		if got.SubRuleRef != domain.RuleOutcomeFail {
			t.Errorf("expected .fail, got %s", got.SubRuleRef)
		}
	})

	t.Run("meta bag still carries non-colliding keys", func(t *testing.T) {
		eng, _ := NewEngine(nil, 5)
		defer func() { _ = eng.Close() }()
		loadClobberRule(t, eng, "meta-country", "has(meta.country) && meta.country == 'US' ? 1.0 : 0.0")

		got := evalClobber(t, eng, map[string]any{"country": "US"})
		if got.Score != 1.0 {
			t.Errorf("expected score 1.0 (meta.country == 'US'), got %.2f", got.Score)
		}
		if got.SubRuleRef != domain.RuleOutcomeFail {
			t.Errorf("expected .fail, got %s", got.SubRuleRef)
		}
	})

	t.Run("meta nil key does not break has(meta.x) guard", func(t *testing.T) {
		eng, _ := NewEngine(nil, 5)
		defer func() { _ = eng.Close() }()
		loadClobberRule(t, eng, "meta-nil", "has(meta.country) && meta.country == 'US' ? 1.0 : 0.0")

		// A caller sending metadata: {"meta": null} must not break the meta bag
		// (the bag is re-asserted after the merge).
		got := evalClobber(t, eng, map[string]any{"meta": nil, "country": "US"})
		if got.SubRuleRef == domain.RuleOutcomeError {
			t.Errorf("meta bag must not error when caller sends meta=null: %s", got.Reason)
		}
		if got.Score != 1.0 {
			t.Errorf("expected score 1.0 (meta.country == 'US' survives meta=nil), got %.2f", got.Score)
		}
	})

	t.Run("enrichment nil key does not break has(enrichment.x) guard", func(t *testing.T) {
		eng, _ := NewEngine(nil, 5)
		defer func() { _ = eng.Close() }()
		loadClobberRule(t, eng, "enrichment-ml", "has(enrichment.ml_score) && enrichment.ml_score > 0.9 ? 1.0 : 0.0")

		ctx := context.Background()
		input := &EvaluateInput{
			TenantID:   "t",
			TxID:       "x",
			Enrichment: map[string]any{"ml_score": 0.92},
			// Attacker tries to null the enrichment bag via metadata.
			AdditionalData: map[string]any{"enrichment": nil},
		}
		results, err := eng.EvaluateAll(ctx, input)
		if err != nil {
			t.Fatalf("EvaluateAll: %v", err)
		}
		if results[0].SubRuleRef == domain.RuleOutcomeError {
			t.Errorf("enrichment bag must not error when caller sends enrichment=null: %s", results[0].Reason)
		}
		if results[0].Score != 1.0 {
			t.Errorf("expected score 1.0 (enrichment.ml_score survives), got %.2f", results[0].Score)
		}
	})
}

// TestMetadataClobberBackcompat asserts the two intended metadata-overridable
// Catalog variables (old_balance/new_balance) still override their 0.0 defaults
// when supplied via metadata. This is the legitimate back-compat path the
// unfiltered merge existed to support; the allow-list must preserve it.
func TestMetadataClobberBackcompat(t *testing.T) {
	t.Run("old_balance/new_balance override defaults via metadata", func(t *testing.T) {
		eng, _ := NewEngine(nil, 5)
		defer func() { _ = eng.Close() }()
		loadClobberRule(t, eng, "drain", "old_balance > 0.0 && new_balance == 0.0 ? 1.0 : 0.0")

		got := evalClobber(t, eng, map[string]any{"old_balance": 15000.0, "new_balance": 0.0})
		if got.Score != 1.0 {
			t.Errorf("expected score 1.0 (old_balance=15000, new_balance=0), got %.2f", got.Score)
		}
		if got.SubRuleRef != domain.RuleOutcomeFail {
			t.Errorf("expected .fail, got %s", got.SubRuleRef)
		}
	})

	t.Run("old_balance/new_balance default to 0.0 without metadata", func(t *testing.T) {
		eng, _ := NewEngine(nil, 5)
		defer func() { _ = eng.Close() }()
		loadClobberRule(t, eng, "drain-default", "old_balance > 0.0 && new_balance == 0.0 ? 1.0 : 0.0")

		got := evalClobber(t, eng, map[string]any{})
		if got.Score != 0.0 {
			t.Errorf("expected score 0.0 (defaults), got %.2f", got.Score)
		}
		if got.SubRuleRef != domain.RuleOutcomePass {
			t.Errorf("expected .pass, got %s", got.SubRuleRef)
		}
	})

	t.Run("new_balance alone overrides default", func(t *testing.T) {
		eng, _ := NewEngine(nil, 5)
		defer func() { _ = eng.Close() }()
		loadClobberRule(t, eng, "newbal", "new_balance > 1000.0 ? 1.0 : 0.0")

		got := evalClobber(t, eng, map[string]any{"new_balance": 5000.0})
		if got.Score != 1.0 {
			t.Errorf("expected score 1.0 (new_balance=5000), got %.2f", got.Score)
		}
		if got.SubRuleRef != domain.RuleOutcomeFail {
			t.Errorf("expected .fail, got %s", got.SubRuleRef)
		}
	})
}

// TestMetadataOverridableDerivedFromCatalog asserts the allow-list is the single
// source of truth: it must contain exactly the Catalog variables marked
// metadataSourced (old_balance, new_balance) and nothing else — in particular
// none of the engine-authoritative names.
func TestMetadataOverridableDerivedFromCatalog(t *testing.T) {
	want := map[string]struct{}{
		"old_balance": {},
		"new_balance": {},
	}
	if len(metadataOverridable) != len(want) {
		t.Fatalf("metadataOverridable has %d entries, want %d", len(metadataOverridable), len(want))
	}
	for k := range want {
		if _, ok := metadataOverridable[k]; !ok {
			t.Errorf("expected %q in metadataOverridable", k)
		}
	}
	// Authoritative names must NOT be overridable.
	authoritative := []string{
		"amount", "currency", "tx_type", "debtor_id", "creditor_id", "tx",
		"velocity_count", "velocity_amount_sum", "velocity_distinct_creditors",
		"meta", "enrichment",
	}
	for _, name := range authoritative {
		if _, ok := metadataOverridable[name]; ok {
			t.Errorf("engine-authoritative variable %q must NOT be metadata-overridable", name)
		}
	}
}
