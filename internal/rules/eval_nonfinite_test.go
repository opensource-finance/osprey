package rules

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// TestEvaluateRuleNonFiniteScoreFailsSecure is the primary regression test for
// the non-finite-score bug. In cel-go, double division by zero does not error:
// 0.0/0.0 yields NaN and x/0.0 yields +/-Inf with err=nil (only integer
// division by zero raises a runtime error). Before the fix, toScore passed
// NaN/Inf through untouched, matchBand matched no band for NaN (all
// comparisons are false) and returned the default .pass "no matching band"
// (a silent false-negative), and the non-finite Score broke evaluation
// persistence because encoding/json rejects NaN/Inf. After the fix, a
// non-finite score fails secure to .err so downstream TADP forces ALRT,
// matching the .err contract in docs/RULE_TYPOLOGY_AUTHORING.md.
func TestEvaluateRuleNonFiniteScoreFailsSecure(t *testing.T) {
	ctx := context.Background()

	zero := 0.0
	one := 1.0

	tests := []struct {
		name       string
		expression string
		input      *EvaluateInput
		wantSubstr string
	}{
		{
			// Realistic account-drain ratio: both balances default to 0.0 in
			// the activation map when the caller omits them, so 0.0/0.0 -> NaN.
			name:       "NaN from new_balance / old_balance both defaulting to 0.0",
			expression: "new_balance / old_balance",
			input:      &EvaluateInput{TenantID: "t", TxID: "x"},
			wantSubstr: "non-finite",
		},
		{
			name:       "NaN from literal 0.0 / 0.0",
			expression: "0.0 / 0.0",
			input:      &EvaluateInput{TenantID: "t", TxID: "x"},
			wantSubstr: "non-finite",
		},
		{
			// Realistic spend-spike ratio: a first-ever transaction has a zero
			// velocity_amount_sum aggregate, so amount/0.0 -> +Inf.
			name:       "plus Inf from tx.amount / 0.0",
			expression: "tx.amount / 0.0",
			input:      &EvaluateInput{TenantID: "t", TxID: "x", Amount: 500.0},
			wantSubstr: "non-finite",
		},
		{
			name:       "plus Inf from literal 1.0 / 0.0",
			expression: "1.0 / 0.0",
			input:      &EvaluateInput{TenantID: "t", TxID: "x"},
			wantSubstr: "non-finite",
		},
		{
			name:       "minus Inf from literal -1.0 / 0.0",
			expression: "-1.0 / 0.0",
			input:      &EvaluateInput{TenantID: "t", TxID: "x"},
			wantSubstr: "non-finite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, _ := NewEngine(nil, 5)
			defer func() { _ = engine.Close() }()

			rule := &domain.RuleConfig{
				ID:         "drain-ratio",
				Name:       "Account Drain Ratio",
				Expression: tt.expression,
				// Bands that would catch a finite ratio. NaN must not match the
				// high band, and must not fall through to .pass either.
				Bands: []domain.RuleBand{
					{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal ratio"},
					{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeFail, Reason: "Drain ratio"},
				},
				Weight:  1.0,
				Enabled: true,
			}
			if err := engine.LoadRule(rule); err != nil {
				t.Fatalf("rule should compile (the non-finite value is a runtime result), but load failed: %v", err)
			}

			res, err := engine.EvaluateAll(ctx, tt.input)
			if err != nil {
				t.Fatalf("a per-rule non-finite score must not fail EvaluateAll, got: %v", err)
			}
			if len(res) != 1 {
				t.Fatalf("expected one result, got %d", len(res))
			}
			r := res[0]
			if r.SubRuleRef != domain.RuleOutcomeError {
				t.Errorf("BUG: non-finite score returned %s (expected .err). Before the fix NaN "+
					"fell through matchBand to .pass \"no matching band\" (silent false-negative) and "+
					"+Inf matched the open high band as .fail but still broke persistence.", r.SubRuleRef)
			}
			if !strings.Contains(r.Reason, tt.wantSubstr) {
				t.Errorf("expected reason to contain %q, got: %s", tt.wantSubstr, r.Reason)
			}
			// The non-finite value must NOT leak into the persisted Score field.
			// Leave it at the zero value so the result stays JSON-encodable.
			if math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
				t.Errorf("BUG: non-finite Score=%.17f leaked into RuleResult.Score. This breaks "+
					"encoding/json and drops the whole evaluation at persistence.", r.Score)
			}
			if r.Score != 0.0 {
				t.Errorf("non-finite rule Score=%.17f, want 0.0 (the fail-secure zero value)", r.Score)
			}
			if r.ProcessMs < 0 {
				t.Error("ProcessMs should be non-negative")
			}
		})
	}
}

// TestNonFiniteScoreStaysJSONEncodable verifies the persistence-breaking half
// of the bug is fixed. Before the fix, a non-finite Score in RuleResult caused
// json.Marshal to fail with "json: unsupported value: NaN" inside
// internal/repository/repository.go SaveEvaluation, which produced HTTP 500 on
// POST /evaluate after the transaction row was already committed, and silently
// dropped the message on the async worker path.
func TestNonFiniteScoreStaysJSONEncodable(t *testing.T) {
	ctx := context.Background()
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	rule := &domain.RuleConfig{
		ID:         "nan-rule",
		Name:       "NaN Rule",
		Expression: "new_balance / old_balance", // both default to 0.0 -> NaN
		Weight:     1.0,
		Enabled:    true,
	}
	if err := engine.LoadRule(rule); err != nil {
		t.Fatalf("load rule: %v", err)
	}

	res, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "x"})
	if err != nil {
		t.Fatalf("EvaluateAll: %v", err)
	}

	// This is the exact call repository.SaveEvaluation makes on eval.RuleResults.
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("BUG: json.Marshal of rule results failed: %v. Before the fix a NaN Score "+
			"made evaluation persistence fail and the whole evaluation was lost.", err)
	}

	// Round-trip back and re-assert the fail-secure outcome survived storage.
	var back []domain.RuleResult
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("json.Unmarshal round-trip: %v", err)
	}
	if len(back) != 1 || back[0].SubRuleRef != domain.RuleOutcomeError {
		t.Fatalf("round-trip lost the .err outcome, got %+v", back)
	}
	if math.IsNaN(back[0].Score) || math.IsInf(back[0].Score, 0) {
		t.Fatalf("BUG: non-finite Score survived the JSON round-trip: %.17f", back[0].Score)
	}
}

// TestNonFiniteScoreForcesAlertThroughTADP is the end-to-end guarantee. A
// non-finite rule result must drive the TADP decision processor to ALRT, not
// NALT, because .err sets HasCriticalFailure which forces an alert. Before the
// fix, the NaN rule was .pass and contributed NaN to the aggregate score, so
// the decision was a silent NALT false-negative. The produced evaluation must
// also stay JSON-encodable end to end.
func TestNonFiniteScoreForcesAlertThroughTADP(t *testing.T) {
	ctx := context.Background()
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	zero := 0.0
	one := 1.0

	// Rule A: NaN-producing ratio. Weight 0.3 keeps its zero contribution to
	// the aggregate below the default AlertThreshold of 0.7, so the alert can
	// only come from the .err force-alert path, not the aggregate score.
	ruleA := &domain.RuleConfig{
		ID:         "nan-drain-ratio",
		Name:       "NaN Drain Ratio",
		Expression: "new_balance / old_balance", // both default to 0.0 -> NaN
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
			{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeFail, Reason: "Drain"},
		},
		Weight:  0.3,
		Enabled: true,
	}
	if err := engine.LoadRule(ruleA); err != nil {
		t.Fatalf("load ruleA: %v", err)
	}

	// Rule B: an always-pass baseline with weight 0.7 so the aggregate is
	// 0.0 (below the 0.7 threshold). The alert must come from the .err path.
	ruleB := &domain.RuleConfig{
		ID:         "baseline-pass",
		Name:       "Baseline Pass",
		Expression: "false",
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, SubRuleRef: domain.RuleOutcomePass, Reason: "Baseline"},
		},
		Weight:  0.7,
		Enabled: true,
	}
	if err := engine.LoadRule(ruleB); err != nil {
		t.Fatalf("load ruleB: %v", err)
	}

	results, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t1", TxID: "tx1"})
	if err != nil {
		t.Fatalf("EvaluateAll: %v", err)
	}

	byID := map[string]domain.RuleResult{}
	for _, r := range results {
		byID[r.RuleID] = r
	}
	if byID["nan-drain-ratio"].SubRuleRef != domain.RuleOutcomeError {
		t.Fatalf("nan-drain-ratio must be .err, got %s", byID["nan-drain-ratio"].SubRuleRef)
	}

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

	if decision.Status != domain.StatusAlert {
		t.Errorf("BUG: Status=%s (expected %s). Before the fix the NaN rule was .pass and the "+
			"aggregate stayed below 0.7, so the decision was a silent NALT false-negative. The .err "+
			"force-alert path was lost because matchBand banded NaN as .pass.", decision.Status, domain.StatusAlert)
	}
	// The aggregate score must stay finite, not become NaN from a NaN
	// contribution leaking through the .pass branch.
	if math.IsNaN(decision.Score) || math.IsInf(decision.Score, 0) {
		t.Errorf("BUG: decision Score is non-finite (%.17f). A NaN rule contribution leaked into "+
			"the aggregate score.", decision.Score)
	}

	// The whole evaluation (rule results + typology results + metadata) must
	// marshal: SaveEvaluation does this in three json.Marshal calls.
	if _, err := json.Marshal(decision); err != nil {
		t.Errorf("BUG: json.Marshal of the evaluation failed: %v", err)
	}
	if _, err := json.Marshal(decision.RuleResults); err != nil {
		t.Errorf("BUG: json.Marshal of rule results failed: %v", err)
	}
	if _, err := json.Marshal(decision.TypologyResults); err != nil {
		t.Errorf("BUG: json.Marshal of typology results failed: %v", err)
	}
}

// TestFiniteDoubleScoreUnaffectedByNonFiniteGuard is the no-regression guard
// for finite double expressions. A non-finite check must not trigger on real
// fractional scores, including a division by a nonzero constant (the only
// shipped division expression, amount/1000.0, in configs/rules/fatf-rules.json).
func TestFiniteDoubleScoreUnaffectedByNonFiniteGuard(t *testing.T) {
	ctx := context.Background()
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	zero, half, one := 0.0, 0.5, 1.0
	rule := &domain.RuleConfig{
		ID:         "finite-ratio",
		Name:       "Finite Ratio",
		Expression: "amount / 1000.0", // amount=500 -> 0.5; amount=2000 -> 2.0
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
			{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeReview, Reason: "Elevated"},
			{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeFail, Reason: "High"},
		},
		Weight:  1.0,
		Enabled: true,
	}
	if err := engine.LoadRule(rule); err != nil {
		t.Fatalf("load rule: %v", err)
	}

	for _, tc := range []struct {
		name   string
		amount float64
		want   float64
		ref    string
	}{
		{"250/1000 = 0.25 -> .pass", 250.0, 0.25, domain.RuleOutcomePass},
		{"800/1000 = 0.8 -> .review", 800.0, 0.8, domain.RuleOutcomeReview},
		{"2000/1000 = 2.0 -> .fail (open last band)", 2000.0, 2.0, domain.RuleOutcomeFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := engine.EvaluateAll(ctx, &EvaluateInput{
				TenantID: "t", TxID: "x", Amount: tc.amount,
			})
			if err != nil {
				t.Fatalf("EvaluateAll: %v", err)
			}
			if len(res) != 1 {
				t.Fatalf("expected 1 result, got %d", len(res))
			}
			r := res[0]
			if math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
				t.Fatalf("BUG: finite expression produced non-finite Score=%.17f", r.Score)
			}
			if r.Score != tc.want {
				t.Errorf("Score=%.17f, want %.1f (finite double scoring must be unchanged)", r.Score, tc.want)
			}
			if r.SubRuleRef != tc.ref {
				t.Errorf("SubRuleRef=%s, want %s (band matching must be unchanged for finite scores)", r.SubRuleRef, tc.ref)
			}
		})
	}
}
