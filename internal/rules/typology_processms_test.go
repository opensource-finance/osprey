package rules

import (
	"fmt"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// makeIdenticalTypologies builds n enabled typologies that each reference the
// same ruleIDs with identical weights, so every typology performs the same
// amount of work during evaluation. This isolates timing behavior from the
// work-per-typology dimension: under a correct per-typology timing impl every
// result reports the same (small) ProcessMs, whereas under the cumulative bug
// the reported ProcessMs grows with iteration position.
func makeIdenticalTypologies(n int, ruleIDs []string) []*domain.Typology {
	weights := make([]domain.TypologyRuleWeight, len(ruleIDs))
	for i, id := range ruleIDs {
		// Equal weights so the score is deterministic; the threshold is set
		// high enough that none trigger (timing is the focus, not outcomes).
		weights[i] = domain.TypologyRuleWeight{RuleID: id, Weight: 1.0 / float64(len(ruleIDs))}
	}
	typologies := make([]*domain.Typology, n)
	for i := range n {
		typologies[i] = &domain.Typology{
			ID:             fmt.Sprintf("typology-%06d", i),
			Name:           fmt.Sprintf("Typology %d", i),
			AlertThreshold: 2.0,
			Enabled:        true,
			Rules:          weights,
		}
	}
	return typologies
}

// ruleResultsFor returns RuleResults scoring 1.0 for every supplied ruleID.
func ruleResultsFor(ruleIDs []string) []domain.RuleResult {
	rr := make([]domain.RuleResult, len(ruleIDs))
	for i, id := range ruleIDs {
		rr[i] = domain.RuleResult{RuleID: id, Score: 1.0}
	}
	return rr
}

// TestTypologyEngine_ProcessMsPerTypologyNotCumulative is the regression test
// for the cumulative-since-batch-start timing bug in EvaluateTypologies.
//
// Every typology performs identical work, so a correct per-typology timing
// implementation reports a ProcessMs that reflects only that single typology's
// (sub-millisecond, ms-truncated) cost. The reported values therefore sum to
// at most the batch wall-time (in a sequential loop, the sum of the per-item
// wall-times cannot exceed the total wall-time, and integer-ms truncaction only
// reduces them). Only a tiny handful of typologies can report a nonzero value
// (those that happened to straddle a millisecond boundary under scheduler or
// GC noise).
//
// The buggy implementation captures start once before the loop, so each
// typology reports the cumulative time since the batch start. The values then
// form an arithmetic series averaging ~n/2 * (per-typology cost) and summing
// to ~n/2 * batchWallTime — astronomically larger than batchWallTime at scale —
// and the vast majority of typologies report a nonzero value.
//
// This yields two robust, scheduler-noise-insensitive invariants that hold on
// the fixed code and are massively violated on the buggy code:
//
//  1. sum(ProcessMs) <= batchWallTime            (sequential-loop tautology)
//  2. count(ProcessMs > 2ms) < n/100             (few straddle a ms boundary)
//
// Both are invariant to single-iteration scheduling spikes (which inflate the
// spike's ProcessMs and batchWallTime together), unlike a max-or-spread ratio
// check, which is why these are used as the hard gates. min/max/spread are
// logged for diagnostic visibility (the bug report's signature is
// "spread ≈ full batch wall-time").
//
// The scale is chosen so the batch wall-time is comfortably above 1 ms even on
// fast hardware, guaranteeing the bug is observable (cumulative values truncate
// to nonzero and would be surfaced in API JSON by omitempty).
func TestTypologyEngine_ProcessMsPerTypologyNotCumulative(t *testing.T) {
	const nTypologies = 200_000
	ruleIDs := []string{"r1", "r2", "r3", "r4", "r5"}

	engine := NewTypologyEngine()
	engine.LoadTypologies(makeIdenticalTypologies(nTypologies, ruleIDs))
	rr := ruleResultsFor(ruleIDs)

	batchStart := time.Now()
	results := engine.EvaluateTypologies(rr)
	batchWall := time.Since(batchStart).Milliseconds()

	if len(results) != nTypologies {
		t.Fatalf("expected %d results, got %d", nTypologies, len(results))
	}

	var min, max, sum int64
	var nonzeroCount, oversizeCount int
	for i, r := range results {
		if r.ProcessMs < 0 {
			t.Fatalf("result %d (%s) has negative ProcessMs %d", i, r.TypologyID, r.ProcessMs)
		}
		if i == 0 {
			min, max = r.ProcessMs, r.ProcessMs
		}
		if r.ProcessMs < min {
			min = r.ProcessMs
		}
		if r.ProcessMs > max {
			max = r.ProcessMs
		}
		sum += r.ProcessMs
		if r.ProcessMs > 0 {
			nonzeroCount++
		}
		if r.ProcessMs > 2 {
			oversizeCount++
		}
	}
	spread := max - min

	t.Logf("ProcessMs: min=%d max=%d spread=%d sum=%d nonzero=%d (>2ms)=%d  batch wall=%dms (n=%d)",
		min, max, spread, sum, nonzeroCount, oversizeCount, batchWall, nTypologies)

	// The batch must be large enough for the bug to be observable; otherwise the
	// test cannot discriminate (everything truncates to 0 on both implementations).
	if batchWall < 5 {
		t.Fatalf("batch wall-time %dms too small to discriminate the bug; increase nTypologies", batchWall)
	}

	// Invariant 1: sum of per-typology ms cannot exceed the batch wall-time on a
	// correct sequential implementation. The cumulative bug produces a sum of
	// ~(n/2)*batchWall, orders of magnitude larger.
	if sum > batchWall {
		t.Fatalf("sum(ProcessMs)=%dms exceeds batch wall-time=%dms: cumulative-since-start timing, "+
			"not per-typology (expected sum <= batchWall for sequential per-typology timing)", sum, batchWall)
	}

	// Invariant 2: only a handful of typologies can straddle a millisecond
	// boundary on a correct implementation (bounded above by sum/3, in turn
	// bounded by batchWall). The cumulative bug makes the large majority of
	// typologies report a value > 2ms. The n/100 threshold sits comfortably in
	// the several-orders-of-magnitude gap between the two behaviors.
	oversizeLimit := nTypologies / 100
	if oversizeCount >= oversizeLimit {
		t.Fatalf("count(ProcessMs>2ms)=%d >= %d: cumulative-since-start timing produces nearly-all-nonzero "+
			"values, not per-typology", oversizeCount, oversizeLimit)
	}
}

// TestTypologyEngine_EvaluateTypologySingularReportsProcessMs verifies the
// single-typology entry point (EvaluateTypology) preserves Score/Triggered/
// Contributions semantics and measures per-typology timing consistently with
// the batch entry point, rather than leaving ProcessMs at its zero value unset.
//
// The timing consistency check is self-calibrating: the batch entry point (with
// a single typology loaded) measures the same one-typology evaluation the
// singular entry point measures. The two must therefore agree on magnitude. If
// the batch measurement registers a nonzero value (the eval is slow enough to
// cross the 1 ms truncation threshold), the singular measurement must register
// a nonzero value too. This holds regardless of absolute hardware speed — a
// very fast machine simply sees both report 0 and the strict sub-check is
// vacuous, while the correctness guards below still run.
func TestTypologyEngine_EvaluateTypologySingularReportsProcessMs(t *testing.T) {
	typologies := []*domain.Typology{
		{
			ID:             "at",
			Name:           "Account Takeover",
			AlertThreshold: 0.6,
			Enabled:        true,
			Rules: []domain.TypologyRuleWeight{
				{RuleID: "account-drain-001", Weight: 0.4},
				{RuleID: "high-value-001", Weight: 0.25},
				{RuleID: "rapid-movement-001", Weight: 0.2},
				{RuleID: "tx-type-risk-001", Weight: 0.15},
			},
		},
		{
			ID:             "disabled",
			Name:           "Disabled",
			AlertThreshold: 0.5,
			Enabled:        false,
			Rules: []domain.TypologyRuleWeight{
				{RuleID: "rule-1", Weight: 1.0},
			},
		},
	}
	engine := NewTypologyEngine()
	engine.LoadTypologies(typologies)

	rr := []domain.RuleResult{
		{RuleID: "account-drain-001", Score: 1.0},
		{RuleID: "high-value-001", Score: 1.0},
		{RuleID: "rapid-movement-001", Score: 1.0},
		{RuleID: "tx-type-risk-001", Score: 0.3},
	}

	got, ok := engine.EvaluateTypology("at", rr)
	if !ok {
		t.Fatal("expected typology 'at' to exist")
	}
	if got.ProcessMs < 0 {
		t.Fatalf("singular EvaluateTypology reported negative ProcessMs=%d", got.ProcessMs)
	}

	// Score: 1.0*0.4 + 1.0*0.25 + 1.0*0.2 + 0.3*0.15 = 0.895
	if got.Score < 0.894 || got.Score > 0.896 {
		t.Errorf("singular EvaluateTypology score: expected ~0.895, got %v", got.Score)
	}
	if !got.Triggered {
		t.Errorf("singular EvaluateTypology: expected Triggered=true (0.895 >= 0.6), got false")
	}
	if len(got.Contributions) != 4 {
		t.Errorf("singular EvaluateTypology: expected 4 contributions, got %d", len(got.Contributions))
	}

	// Disabled/unknown typology returns (nil, false) and must not panic.
	if _, ok := engine.EvaluateTypology("disabled", rr); ok {
		t.Error("disabled typology should not be returned by EvaluateTypology")
	}
	if _, ok := engine.EvaluateTypology("does-not-exist", rr); ok {
		t.Error("unknown typology should not be found")
	}

	// Self-calibrating timing-consistency check. Use a genuinely slow
	// single-typology (many distinct ruleIDs -> cache-cold map lookups) so its
	// evaluation crosses the 1 ms truncation threshold on realistic hardware.
	// The batch path (proven per-typology by the test above) measures the same
	// one-typology evaluation the singular path measures; they must agree.
	const slowRules = 200_000
	slowRuleIDs := make([]string, slowRules)
	for i := range slowRuleIDs {
		slowRuleIDs[i] = fmt.Sprintf("slow-r-%d", i)
	}
	slowWeights := make([]domain.TypologyRuleWeight, slowRules)
	w := 1.0 / float64(slowRules)
	for i, id := range slowRuleIDs {
		slowWeights[i] = domain.TypologyRuleWeight{RuleID: id, Weight: w}
	}
	slowEngine := NewTypologyEngine()
	slowEngine.LoadTypologies([]*domain.Typology{
		{ID: "slow", Name: "Slow", AlertThreshold: 2.0, Enabled: true, Rules: slowWeights},
	})
	slowRR := ruleResultsFor(slowRuleIDs)

	batchOne := slowEngine.EvaluateTypologies(slowRR)
	if len(batchOne) != 1 {
		t.Fatalf("expected 1 result from single-typology batch, got %d", len(batchOne))
	}
	batchMs := batchOne[0].ProcessMs

	singularGot, singularOK := slowEngine.EvaluateTypology("slow", slowRR)
	if !singularOK {
		t.Fatal("expected slow typology to exist")
	}
	if singularGot.ProcessMs < 0 {
		t.Fatalf("singular EvaluateTypology on slow typology reported negative ProcessMs=%d", singularGot.ProcessMs)
	}
	if len(singularGot.Contributions) != slowRules {
		t.Errorf("slow typology contributions: expected %d, got %d", slowRules, len(singularGot.Contributions))
	}

	// If the shared one-typology evaluation is slow enough to register a nonzero
	// per-typology value via the batch path, the singular path must register a
	// nonzero value too. On the buggy singular path (ProcessMs left unset) this
	// fails; on the fixed path both measurements reflect the same eval.
	if batchMs >= 1 && singularGot.ProcessMs < 1 {
		t.Fatalf("singular EvaluateTypology reported ProcessMs=%d while the batch single-typology "+
			"measurement of the same typology reported ProcessMs=%d; the singular path must measure "+
			"timing consistently with the batch path, not leave ProcessMs unset",
			singularGot.ProcessMs, batchMs)
	}
	t.Logf("slow typology: batch single-typology ProcessMs=%d, singular ProcessMs=%d", batchMs, singularGot.ProcessMs)
}
