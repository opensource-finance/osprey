package rules

import (
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

// TestTypologyEngine_DuplicateRuleIDsScoreOnce is the direct regression test for
// the duplicate-rule double-count bug. A typology that lists the same ruleId
// twice must add that rule's contribution once, not once per entry. With
// threshold 0.6, rules [{R1, 0.5}, {R1, 0.5}] and R1 scoring 1.0, the intended
// score is 0.5 (not triggered). Before the fix each entry added 0.5, the score
// became 1.0, and the typology falsely triggered at the 0.6 threshold.
func TestTypologyEngine_DuplicateRuleIDsScoreOnce(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{{
		ID:             "dup-typology",
		Name:           "Dup Typology",
		AlertThreshold: 0.6,
		Enabled:        true,
		Rules: []domain.TypologyRuleWeight{
			{RuleID: "R1", Weight: 0.5},
			{RuleID: "R1", Weight: 0.5},
		},
	}})

	results := engine.EvaluateTypologies([]domain.RuleResult{
		{RuleID: "R1", Score: 1.0},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if r.Score < 0.499 || r.Score > 0.501 {
		t.Fatalf("expected score ~0.5 (single contribution), got %v", r.Score)
	}
	if r.Triggered {
		t.Fatalf("expected typology NOT triggered (0.5 < 0.6 threshold), got triggered score %v", r.Score)
	}
	if len(r.Contributions) != 1 {
		t.Fatalf("expected 1 contribution for the duplicated rule, got %d", len(r.Contributions))
	}
	c := r.Contributions[0]
	if c.RuleID != "R1" {
		t.Errorf("expected contribution RuleID R1, got %q", c.RuleID)
	}
	if c.Contribution < 0.499 || c.Contribution > 0.501 {
		t.Errorf("expected contribution ~0.5, got %v", c.Contribution)
	}
}

// TestTypologyEngine_DuplicateRuleIDsKeepFirstWeight confirms the engine uses
// the first weight entry for a duplicated rule and ignores later entries. With
// rules [{R1, 0.5}, {R1, 0.3}] and R1=1.0, the contribution must use weight 0.5.
func TestTypologyEngine_DuplicateRuleIDsKeepFirstWeight(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{{
		ID:             "first-weight-typology",
		Name:           "First Weight Typology",
		AlertThreshold: 1.0,
		Enabled:        true,
		Rules: []domain.TypologyRuleWeight{
			{RuleID: "R1", Weight: 0.5},
			{RuleID: "R1", Weight: 0.3},
		},
	}})

	results := engine.EvaluateTypologies([]domain.RuleResult{
		{RuleID: "R1", Score: 1.0},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	if r.Score < 0.499 || r.Score > 0.501 {
		t.Fatalf("expected score ~0.5 (first weight 0.5), got %v", r.Score)
	}
	if len(r.Contributions) != 1 {
		t.Fatalf("expected 1 contribution, got %d", len(r.Contributions))
	}
	if r.Contributions[0].Weight < 0.499 || r.Contributions[0].Weight > 0.501 {
		t.Errorf("expected kept weight 0.5, got %v", r.Contributions[0].Weight)
	}
}

// TestTypologyEngine_EvaluateTypologySingularDeduplicates confirms the single
// typology evaluation path (EvaluateTypology by ID) also deduplicates, matching
// the batch path. A duplicate must not double-count on either path.
func TestTypologyEngine_EvaluateTypologySingularDeduplicates(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{{
		ID:             "singular-dup",
		Name:           "Singular Dup",
		AlertThreshold: 0.6,
		Enabled:        true,
		Rules: []domain.TypologyRuleWeight{
			{RuleID: "R1", Weight: 0.5},
			{RuleID: "R1", Weight: 0.5},
		},
	}})

	got, ok := engine.EvaluateTypology("singular-dup", []domain.RuleResult{{RuleID: "R1", Score: 1.0}})
	if !ok {
		t.Fatal("expected typology 'singular-dup' to exist")
	}
	if got.Score < 0.499 || got.Score > 0.501 {
		t.Fatalf("expected singular path score ~0.5, got %v", got.Score)
	}
	if got.Triggered {
		t.Errorf("expected singular path NOT triggered (0.5 < 0.6), got triggered")
	}
	if len(got.Contributions) != 1 {
		t.Errorf("expected 1 contribution on singular path, got %d", len(got.Contributions))
	}
}

// TestTypologyEngine_NonDuplicateTypologyUnchanged confirms the dedup logic does
// not alter scoring for a typology with unique rule IDs (regression guard).
func TestTypologyEngine_NonDuplicateTypologyUnchanged(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{{
		ID:             "unique-typology",
		Name:           "Unique Typology",
		AlertThreshold: 0.7,
		Enabled:        true,
		Rules: []domain.TypologyRuleWeight{
			{RuleID: "R1", Weight: 0.5},
			{RuleID: "R2", Weight: 0.3},
			{RuleID: "R3", Weight: 0.2},
		},
	}})

	results := engine.EvaluateTypologies([]domain.RuleResult{
		{RuleID: "R1", Score: 1.0},
		{RuleID: "R2", Score: 0.5},
		{RuleID: "R3", Score: 0.0},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]

	// 1.0*0.5 + 0.5*0.3 + 0.0*0.2 = 0.5 + 0.15 + 0 = 0.65, NOT triggered (0.65 < 0.7).
	if r.Score < 0.649 || r.Score > 0.651 {
		t.Errorf("expected score ~0.65 for non-duplicate typology, got %v", r.Score)
	}
	if r.Triggered {
		t.Errorf("expected NOT triggered (0.65 < 0.7), got triggered")
	}
	if len(r.Contributions) != 3 {
		t.Errorf("expected 3 contributions, got %d", len(r.Contributions))
	}
}

// TestTypologyEngine_LoadTypologiesAcceptsDuplicates confirms that a typology
// with duplicate rule IDs is still loaded (the engine fail-closed by keeping
// it) and the load-time warning does not panic or drop the typology. The
// defensive dedup at evaluation keeps the score correct.
func TestTypologyEngine_LoadTypologiesAcceptsDuplicates(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{{
		ID:             "load-dup",
		Name:           "Load Dup",
		AlertThreshold: 0.6,
		Enabled:        true,
		Rules: []domain.TypologyRuleWeight{
			{RuleID: "R1", Weight: 0.5},
			{RuleID: "R1", Weight: 0.5},
		},
	}})

	if engine.TypologyCount() != 1 {
		t.Fatalf("expected the duplicated typology to still be loaded (1), got %d", engine.TypologyCount())
	}

	loaded := engine.GetLoadedTypologies()
	if len(loaded) != 1 || loaded[0].ID != "load-dup" {
		t.Fatalf("expected load-dup to be present, got %v", loaded)
	}
	// The stored Rules list is not mutated; the dedup happens at evaluation.
	if len(loaded[0].Rules) != 2 {
		t.Errorf("expected the typology Rules list to be unchanged (2 entries), got %d", len(loaded[0].Rules))
	}
}
