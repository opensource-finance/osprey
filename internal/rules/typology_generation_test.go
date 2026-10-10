package rules

import (
	"sync"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestTypologyEngine_GenerationStartsAtZero(t *testing.T) {
	engine := NewTypologyEngine()
	if g := engine.Generation(); g != 0 {
		t.Fatalf("fresh engine generation: expected 0, got %d", g)
	}
}

func TestTypologyEngine_LoadBumpsGeneration(t *testing.T) {
	engine := NewTypologyEngine()

	engine.LoadTypologies([]*domain.Typology{
		{ID: "t1", Name: "T1", AlertThreshold: 0.5, Enabled: true, Rules: []domain.TypologyRuleWeight{{RuleID: "r1", Weight: 1.0}}},
	})
	g1 := engine.Generation()
	if g1 != 1 {
		t.Fatalf("after first load: expected generation 1, got %d", g1)
	}

	// A no-op reload (empty set) must still bump the generation: callers rely
	// on the bump to detect "typologies cleared mid-request".
	engine.LoadTypologies(nil)
	g2 := engine.Generation()
	if g2 != 2 {
		t.Fatalf("after empty reload: expected generation 2, got %d", g2)
	}
	if engine.TypologyCount() != 0 {
		t.Fatalf("after empty reload: expected count 0, got %d", engine.TypologyCount())
	}

	engine.ReloadTypologies([]*domain.Typology{
		{ID: "t2", Name: "T2", AlertThreshold: 0.5, Enabled: true, Rules: []domain.TypologyRuleWeight{{RuleID: "r1", Weight: 1.0}}},
	})
	if g := engine.Generation(); g != 3 {
		t.Fatalf("after reload: expected generation 3, got %d", g)
	}
}

func TestTypologyEngine_SnapshotIsAtomic(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{
		{ID: "t1", Name: "T1", AlertThreshold: 0.5, Enabled: true, Rules: []domain.TypologyRuleWeight{{RuleID: "r1", Weight: 1.0}}},
	})

	gen, count := engine.Snapshot()
	if gen != 1 {
		t.Fatalf("snapshot gen: expected 1, got %d", gen)
	}
	if count != 1 {
		t.Fatalf("snapshot count: expected 1, got %d", count)
	}

	// After clearing, the snapshot gen reflects the bump and count 0.
	engine.LoadTypologies(nil)
	gen, count = engine.Snapshot()
	if gen != 2 {
		t.Fatalf("snapshot gen after clear: expected 2, got %d", gen)
	}
	if count != 0 {
		t.Fatalf("snapshot count after clear: expected 0, got %d", count)
	}
}

func TestTypologyEngine_EvaluateTypologiesIfStable_RejectsOnGenerationChange(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{
		{ID: "structuring-check", Name: "Structuring", AlertThreshold: 0.9, Enabled: true, Rules: []domain.TypologyRuleWeight{{RuleID: "high-amount-rule", Weight: 1.0}}},
	})
	entryGen, count := engine.Snapshot()
	if count != 1 {
		t.Fatalf("entry: expected count 1, got %d", count)
	}

	// A concurrent admin operation empties the set before evaluation.
	engine.LoadTypologies(nil)

	rr := []domain.RuleResult{{RuleID: "high-amount-rule", Score: 0.8}}
	results, ok := engine.EvaluateTypologiesIfStable(rr, entryGen)
	if ok {
		t.Fatal("expected ok=false when generation changed mid-request; got ok=true")
	}
	if results != nil {
		t.Fatalf("expected nil results when generation changed; got %d result(s)", len(results))
	}
}

func TestTypologyEngine_EvaluateTypologiesIfStable_RejectsWhenEmpty(t *testing.T) {
	engine := NewTypologyEngine()
	// Load then clear so the generation is non-zero but the set is empty.
	engine.LoadTypologies([]*domain.Typology{
		{ID: "t1", Name: "T1", AlertThreshold: 0.5, Enabled: true, Rules: []domain.TypologyRuleWeight{{RuleID: "r1", Weight: 1.0}}},
	})
	engine.LoadTypologies(nil)
	gen, count := engine.Snapshot()
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}

	results, ok := engine.EvaluateTypologiesIfStable(nil, gen)
	if ok {
		t.Fatal("expected ok=false when typology set is empty even if generation matches")
	}
	if results != nil {
		t.Fatalf("expected nil results for empty set; got %d result(s)", len(results))
	}
}

func TestTypologyEngine_EvaluateTypologiesIfStable_AcceptsUnchanged(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{
		{ID: "structuring-check", Name: "Structuring", AlertThreshold: 0.9, Enabled: true, Rules: []domain.TypologyRuleWeight{{RuleID: "high-amount-rule", Weight: 1.0}}},
	})
	entryGen, count := engine.Snapshot()
	if count != 1 {
		t.Fatalf("entry: expected count 1, got %d", count)
	}

	rr := []domain.RuleResult{{RuleID: "high-amount-rule", Score: 0.8}}
	results, ok := engine.EvaluateTypologiesIfStable(rr, entryGen)
	if !ok {
		t.Fatal("expected ok=true when typology set is unchanged")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 typology result, got %d", len(results))
	}
	if results[0].TypologyID != "structuring-check" {
		t.Errorf("expected typology 'structuring-check', got %q", results[0].TypologyID)
	}
	// Score 0.8 * weight 1.0 = 0.8 < threshold 0.9 -> not triggered.
	if results[0].Triggered {
		t.Errorf("expected not triggered (0.8 < 0.9), got triggered=true")
	}
}

// TestTypologyEngine_EvaluateTypologiesIfStable_RejectsOnNonEmptyReplacement proves
// the generation check also catches a non-empty replacement (e.g. admin swaps
// the typology set), not just the "cleared to empty" case.
func TestTypologyEngine_EvaluateTypologiesIfStable_RejectsOnNonEmptyReplacement(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{
		{ID: "structuring-check", Name: "Structuring", AlertThreshold: 0.9, Enabled: true, Rules: []domain.TypologyRuleWeight{{RuleID: "high-amount-rule", Weight: 1.0}}},
	})
	entryGen, _ := engine.Snapshot()

	// Replace with a different (non-empty) set.
	engine.LoadTypologies([]*domain.Typology{
		{ID: "totally-different", Name: "Different", AlertThreshold: 0.5, Enabled: true, Rules: []domain.TypologyRuleWeight{{RuleID: "other-rule", Weight: 1.0}}},
	})

	results, ok := engine.EvaluateTypologiesIfStable(nil, entryGen)
	if ok {
		t.Fatal("expected ok=false when typology set was replaced (non-empty) mid-request")
	}
	if results != nil {
		t.Fatalf("expected nil results on replacement; got %d result(s)", len(results))
	}
}

// TestTypologyEngine_GenerationIsConcurrencySafe ensures concurrent loads and
// generation reads race-free (run with -race).
func TestTypologyEngine_GenerationIsConcurrencySafe(t *testing.T) {
	engine := NewTypologyEngine()
	typo := &domain.Typology{
		ID:             "t",
		Name:           "T",
		AlertThreshold: 0.5,
		Enabled:        true,
		Rules:          []domain.TypologyRuleWeight{{RuleID: "r", Weight: 1.0}},
	}
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			engine.LoadTypologies([]*domain.Typology{typo})
			_ = engine.Generation()
			_, _ = engine.Snapshot()
			_, _ = engine.EvaluateTypologiesIfStable(nil, 0)
		})
	}
	wg.Wait()
}
