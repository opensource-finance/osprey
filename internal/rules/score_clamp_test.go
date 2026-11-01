package rules

import (
	"context"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// ============================================================================
// SCORE CLAMP TESTS
//
// Detection rule scores are documented to live in [0, 1] (docs/QUICKSTART.md:
// "Risk score from 0 to 1"), and the aggregation contract in
// docs/ARCHITECTURE.md says "With sum, a rule that does not match adds 0".
// Before the fix, a CEL expression returning a signed numeric passed through
// toScore verbatim: a negative result matched no band (defaulting to .pass)
// but kept its negative value, which was then subtracted from the weighted
// aggregate. One negative-scoring rule could cancel matching rules, flip an
// ALRT to NALT (fail-open), and persist/publish a below-zero score. These
// tests lock the clamp at the score source (evaluateRule) so every downstream
// consumer — matchBand, the aggregate, ScoringAverage, rule_results, the
// published evaluation score — stays in range.
// ============================================================================

// detectionBands is a typical [0, 0.5) -> .pass, [0.5, 1.0] -> .review band set
// with a closed last band at 1.0, used across the clamp tests.
func detectionBands() []domain.RuleBand {
	zero, half, one := 0.0, 0.5, 1.0
	return []domain.RuleBand{
		{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
		{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeReview, Reason: "Elevated"},
	}
}

// TestScoreClampNegativeDoubleIsZero proves the root-cause fix: a CEL
// expression whose double result is negative is clamped to 0.0 (and bands to
// .pass), instead of the prior behavior of passing -2.0 through verbatim.
func TestScoreClampNegativeDoubleIsZero(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	if err := engine.LoadRule(&domain.RuleConfig{
		ID:         "neg-double",
		Expression: "0.5 - amount / 100.0", // amount=250 -> -2.0
		Bands:      detectionBands(),
		Weight:     1.0,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("load: %v", err)
	}

	results, err := engine.EvaluateAll(context.Background(), &EvaluateInput{
		TenantID: "t", TxID: "tx", Amount: 250.0,
	})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Score != 0.0 {
		t.Errorf("negative double must clamp to 0.0, got Score=%.17f (subtracted from aggregate = fail-open)", results[0].Score)
	}
	if results[0].SubRuleRef != domain.RuleOutcomePass {
		t.Errorf("clamped score 0.0 must band to .pass, got %s", results[0].SubRuleRef)
	}
}

// TestScoreClampAboveOneIsOne covers the upper bound: a double result above 1
// must clamp to 1.0 and match the closed last band, not pass through as the
// raw large value.
func TestScoreClampAboveOneIsOne(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	if err := engine.LoadRule(&domain.RuleConfig{
		ID:         "big-double",
		Expression: "amount / 10.0", // amount=5000 -> 500.0
		Bands:      detectionBands(),
		Weight:     1.0,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("load: %v", err)
	}

	results, err := engine.EvaluateAll(context.Background(), &EvaluateInput{
		TenantID: "t", TxID: "tx", Amount: 5000.0,
	})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	if results[0].Score != 1.0 {
		t.Errorf("double above 1 must clamp to 1.0, got Score=%.17f", results[0].Score)
	}
	if results[0].SubRuleRef != domain.RuleOutcomeReview {
		t.Errorf("clamped score 1.0 must match the closed last band (.review), got %s", results[0].SubRuleRef)
	}
}

// TestScoreClampNegativeEnrichmentValue reproduces the realistic production
// trigger from the report: an externally-computed enrichment value flows in
// via the open-ended `enrichment` map, and double(...) wraps it into a
// Double-typed expression the compiler accepts. A negative external value
// (ml/trust score with a signed scale) must clamp to 0.0, not pass through.
func TestScoreClampNegativeEnrichmentValue(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	if err := engine.LoadRule(&domain.RuleConfig{
		ID:         "enrich-trust",
		Expression: "double(enrichment.trust_score)",
		Bands:      detectionBands(),
		Weight:     1.0,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("load: %v", err)
	}

	results, err := engine.EvaluateAll(context.Background(), &EvaluateInput{
		TenantID:   "t",
		TxID:       "tx",
		Enrichment: map[string]any{"trust_score": -0.3},
	})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	if results[0].Score != 0.0 {
		t.Errorf("negative enrichment value must clamp to 0.0, got Score=%.17f (published as rule_results.score)", results[0].Score)
	}
	if results[0].SubRuleRef != domain.RuleOutcomePass {
		t.Errorf("clamped score 0.0 must band to .pass, got %s", results[0].SubRuleRef)
	}
}

// TestScoreClampNoOpForInRangeScores guards against regressions: legitimate
// fractional scores at 0.0, 0.5 and 1.0 (and a mid value) must pass through
// the clamp unchanged, so existing rule sets are unaffected.
func TestScoreClampNoOpForInRangeScores(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	if err := engine.LoadRule(&domain.RuleConfig{
		ID:         "in-range",
		Expression: "amount / 1000.0", // 0.0, 0.5, 1.0, 0.75 for the amounts below
		Bands:      detectionBands(),
		Weight:     1.0,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("load: %v", err)
	}

	for _, tc := range []struct {
		name   string
		amount float64
		want   float64
	}{
		{"zero", 0.0, 0.0},
		{"half", 500.0, 0.5},
		{"three-quarters", 750.0, 0.75},
		{"exactly one", 1000.0, 1.0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results, err := engine.EvaluateAll(context.Background(), &EvaluateInput{
				TenantID: "t", TxID: "tx", Amount: tc.amount,
			})
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			if results[0].Score != tc.want {
				t.Errorf("amount=%.1f: clamp must be a no-op for in-range score, want %.17f, got %.17f",
					tc.amount, tc.want, results[0].Score)
			}
		})
	}
}

// ============================================================================
// END-TO-END: the headline fail-open bug
// ============================================================================

// TestNegativeScoringRuleDoesNotDowngradeAlertEndToEnd is the end-to-end
// reproduction from the bug report. Rule A alone produces an ALRT (score
// 1.0, weight 1.0, bands to .review). Adding rule B — a legitimate-looking
// double expression `0.5 - amount / 100.0` that is negative for amounts > 50
// — used to subtract -2.0 from the aggregate, scoring the transaction at
// -1.0 and flipping it to NALT (fail-open). After the fix, rule B clamps to
// 0.0, adds nothing, and the transaction still alerts with a non-negative
// published score.
func TestNegativeScoringRuleDoesNotDowngradeAlertEndToEnd(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	bands := detectionBands()

	ruleA := &domain.RuleConfig{
		ID:         "high-amount-review",
		Expression: "amount > 100.0 ? 1.0 : 0.0", // amount=250 -> 1.0
		Bands:      bands,
		Weight:     1.0,
		Enabled:    true,
	}
	ruleB := &domain.RuleConfig{
		ID:         "sanction-offset",
		Expression: "0.5 - amount / 100.0", // amount=250 -> -2.0 (clamped to 0.0)
		Bands:      bands,
		Weight:     1.0,
		Enabled:    true,
	}
	if err := engine.LoadRule(ruleA); err != nil {
		t.Fatalf("load A: %v", err)
	}
	if err := engine.LoadRule(ruleB); err != nil {
		t.Fatalf("load B: %v", err)
	}

	ctx := context.Background()
	results, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "tx", Amount: 250.0})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	var ruleARecv, ruleBRecv *domain.RuleResult
	for i := range results {
		switch results[i].RuleID {
		case ruleA.ID:
			ruleARecv = &results[i]
		case ruleB.ID:
			ruleBRecv = &results[i]
		}
	}
	if ruleARecv == nil || ruleBRecv == nil {
		t.Fatalf("missing rule results: A=%v B=%v", ruleARecv != nil, ruleBRecv != nil)
	}

	// Rule A is unaffected: score 1.0, banded .review.
	if ruleARecv.Score != 1.0 || ruleARecv.SubRuleRef != domain.RuleOutcomeReview {
		t.Errorf("rule A: want Score=1.0 .review, got Score=%.17f %s", ruleARecv.Score, ruleARecv.SubRuleRef)
	}
	// Rule B is the fix: the negative value is clamped to 0.0 (a non-matching
	// rule adds 0, per docs/ARCHITECTURE.md) and banded .pass.
	if ruleBRecv.Score != 0.0 {
		t.Errorf("rule B: negative score must clamp to 0.0, got Score=%.17f (this is the value subtracted from the aggregate before the fix)", ruleBRecv.Score)
	}
	if ruleBRecv.SubRuleRef != domain.RuleOutcomePass {
		t.Errorf("rule B: clamped score must band to .pass, got %s", ruleBRecv.SubRuleRef)
	}

	processor := &tadp.Processor{
		AlertThreshold:     0.7,
		UseWeightedScoring: true,
		Scoring:            tadp.ScoringSum,
		Mode:               "detection",
	}
	decision := processor.Process(ctx, &tadp.DecisionInput{
		TenantID:    "t",
		TxID:        "tx",
		RuleResults: results,
		StartTime:   time.Now(),
	})

	// Before the fix: aggregate = 1.0*1.0 + (-2.0)*1.0 = -1.0 < 0.7 -> NALT
	// (fail-open), and eval.Score = -1.0 was persisted and published.
	// After the fix: aggregate = 1.0*1.0 + 0.0*1.0 = 1.0 >= 0.7 -> ALRT.
	if decision.Status != domain.StatusAlert {
		t.Errorf("MISSED ALERT: negative-scoring rule downgraded ALRT to %s (score %.4f). "+
			"A non-matching rule must add 0, not subtract.", decision.Status, decision.Score)
	}
	if decision.Score < 0.0 {
		t.Errorf("published evaluation score must not be negative, got %.4f", decision.Score)
	}
	if decision.Score < 0.7 {
		t.Errorf("aggregate must reach the threshold with the matching rule, got %.4f < 0.7", decision.Score)
	}

	// The detection summary carries the same non-negative score.
	if len(decision.TypologyResults) != 1 {
		t.Fatalf("expected one detection-summary result, got %d", len(decision.TypologyResults))
	}
	summary := decision.TypologyResults[0]
	if summary.Score < 0.0 || summary.Score > 1.0 {
		t.Errorf("detection-summary score must be in [0, 1], got %.4f", summary.Score)
	}
}

// TestNegativeScoringRuleDoesNotProduceNegativeAverageScore confirms the
// clamp also protects ScoringAverage mode. Before the fix, a negative rule
// score divided by TotalWeight produced a below-zero published score with no
// clamp in either direction. After the fix, the clamped 0.0 contribution
// keeps the weighted average non-negative and within the documented range.
func TestNegativeScoringRuleDoesNotProduceNegativeAverageScore(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer func() { _ = engine.Close() }()

	bands := detectionBands()

	ruleA := &domain.RuleConfig{
		ID:         "signal",
		Expression: "amount > 100.0 ? 0.9 : 0.0", // amount=250 -> 0.9
		Bands:      bands,
		Weight:     1.0,
		Enabled:    true,
	}
	ruleB := &domain.RuleConfig{
		ID:         "offset",
		Expression: "0.5 - amount / 100.0", // amount=250 -> -2.0 (clamped to 0.0)
		Bands:      bands,
		Weight:     1.0,
		Enabled:    true,
	}
	if err := engine.LoadRule(ruleA); err != nil {
		t.Fatalf("load A: %v", err)
	}
	if err := engine.LoadRule(ruleB); err != nil {
		t.Fatalf("load B: %v", err)
	}

	ctx := context.Background()
	results, err := engine.EvaluateAll(ctx, &EvaluateInput{TenantID: "t", TxID: "tx", Amount: 250.0})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	// Rule B's negative result must be clamped at the source.
	var ruleBRecv *domain.RuleResult
	for i := range results {
		if results[i].RuleID == ruleB.ID {
			ruleBRecv = &results[i]
		}
	}
	if ruleBRecv == nil {
		t.Fatal("rule B result not found")
	}
	if ruleBRecv.Score != 0.0 {
		t.Fatalf("rule B: negative score must clamp to 0.0, got %.17f", ruleBRecv.Score)
	}

	processor := &tadp.Processor{
		AlertThreshold:     0.7,
		UseWeightedScoring: true,
		Scoring:            tadp.ScoringAverage,
		Mode:               "detection",
	}
	decision := processor.Process(ctx, &tadp.DecisionInput{
		TenantID:    "t",
		TxID:        "tx",
		RuleResults: results,
		StartTime:   time.Now(),
	})

	// Before the fix: (0.9*1.0 + (-2.0)*1.0) / (1.0 + 1.0) = -0.55 (negative,
	// no clamp in average mode). After the fix: (0.9 + 0.0) / 2.0 = 0.45.
	if decision.Score < 0.0 || decision.Score > 1.0 {
		t.Errorf("average-mode published score must stay in [0, 1], got %.4f", decision.Score)
	}
	wantScore := 0.45
	if decision.Score != wantScore {
		t.Errorf("average-mode score: want %.2f, got %.4f (negative rule must not pull the average below zero)", wantScore, decision.Score)
	}
}
