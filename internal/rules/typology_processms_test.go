package rules

import (
	"fmt"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// makeIdenticalTypologies builds n enabled typologies with identical rule weights.
func makeIdenticalTypologies(n int, ruleIDs []string) []*domain.Typology {
	weights := make([]domain.TypologyRuleWeight, len(ruleIDs))
	for i, id := range ruleIDs {
		// Equal weights; high threshold so none trigger.
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

	// The batch must run long enough for cumulative timing to show.
	if batchWall < 5 {
		t.Fatalf("batch wall-time %dms too small to discriminate the bug; increase nTypologies", batchWall)
	}

	// Per-typology times must sum to at most the batch wall time.
	if sum > batchWall {
		t.Fatalf("sum(ProcessMs)=%dms exceeds batch wall-time=%dms: cumulative-since-start timing, "+
			"not per-typology (expected sum <= batchWall for sequential per-typology timing)", sum, batchWall)
	}

	// Only a few typologies may report more than 2ms.
	oversizeLimit := nTypologies / 100
	if oversizeCount >= oversizeLimit {
		t.Fatalf("count(ProcessMs>2ms)=%d >= %d: cumulative-since-start timing produces nearly-all-nonzero "+
			"values, not per-typology", oversizeCount, oversizeLimit)
	}
}

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

	// A slow typology, so timing crosses 1ms; batch and singular paths must agree.
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

	// If the batch path registers >= 1ms, the singular path must too.
	if batchMs >= 1 && singularGot.ProcessMs < 1 {
		t.Fatalf("singular EvaluateTypology reported ProcessMs=%d while the batch single-typology "+
			"measurement of the same typology reported ProcessMs=%d; the singular path must measure "+
			"timing consistently with the batch path, not leave ProcessMs unset",
			singularGot.ProcessMs, batchMs)
	}
	t.Logf("slow typology: batch single-typology ProcessMs=%d, singular ProcessMs=%d", batchMs, singularGot.ProcessMs)
}
