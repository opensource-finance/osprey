package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestTypologyEngine_EvaluateTypologies(t *testing.T) {
	engine := NewTypologyEngine()

	// Load test typologies
	typologies := []*domain.Typology{
		{
			ID:             "account-takeover",
			Name:           "Account Takeover",
			Description:    "Detects account takeover patterns",
			Version:        "1.0.0",
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
			ID:             "structuring",
			Name:           "Structuring",
			Description:    "Detects structuring patterns",
			Version:        "1.0.0",
			AlertThreshold: 0.5,
			Enabled:        true,
			Rules: []domain.TypologyRuleWeight{
				{RuleID: "structuring-001", Weight: 0.5},
				{RuleID: "round-amount-001", Weight: 0.3},
				{RuleID: "velocity-check-001", Weight: 0.2},
			},
		},
	}

	engine.LoadTypologies(typologies)

	if engine.TypologyCount() != 2 {
		t.Errorf("Expected 2 typologies, got %d", engine.TypologyCount())
	}

	tests := []struct {
		name                string
		ruleResults         []domain.RuleResult
		wantAccountTakeover bool
		wantStructuring     bool
	}{
		{
			name: "Account takeover triggers - all rules fire",
			ruleResults: []domain.RuleResult{
				{RuleID: "account-drain-001", Score: 1.0},  // 0.4
				{RuleID: "high-value-001", Score: 1.0},     // 0.25
				{RuleID: "rapid-movement-001", Score: 1.0}, // 0.2
				{RuleID: "tx-type-risk-001", Score: 0.3},   // 0.045
			},
			wantAccountTakeover: true, // 0.4 + 0.25 + 0.2 + 0.045 = 0.895 >= 0.6
			wantStructuring:     false,
		},
		{
			name: "Account takeover triggers - partial rules",
			ruleResults: []domain.RuleResult{
				{RuleID: "account-drain-001", Score: 1.0}, // 0.4
				{RuleID: "high-value-001", Score: 1.0},    // 0.25
			},
			wantAccountTakeover: true, // 0.4 + 0.25 = 0.65 >= 0.6
			wantStructuring:     false,
		},
		{
			name: "Account takeover does NOT trigger - below threshold",
			ruleResults: []domain.RuleResult{
				{RuleID: "account-drain-001", Score: 0.5}, // 0.2
				{RuleID: "high-value-001", Score: 1.0},    // 0.25
			},
			wantAccountTakeover: false, // 0.2 + 0.25 = 0.45 < 0.6
			wantStructuring:     false,
		},
		{
			name: "Structuring triggers",
			ruleResults: []domain.RuleResult{
				{RuleID: "structuring-001", Score: 0.9},  // 0.45
				{RuleID: "round-amount-001", Score: 0.3}, // 0.09
			},
			wantAccountTakeover: false,
			wantStructuring:     true, // 0.45 + 0.09 = 0.54 >= 0.5
		},
		{
			name: "Both typologies trigger",
			ruleResults: []domain.RuleResult{
				// Account takeover rules
				{RuleID: "account-drain-001", Score: 1.0},
				{RuleID: "high-value-001", Score: 1.0},
				{RuleID: "rapid-movement-001", Score: 1.0},
				{RuleID: "tx-type-risk-001", Score: 0.3},
				// Structuring rules
				{RuleID: "structuring-001", Score: 0.9},
				{RuleID: "round-amount-001", Score: 0.7},
				{RuleID: "velocity-check-001", Score: 1.0},
			},
			wantAccountTakeover: true,
			wantStructuring:     true,
		},
		{
			name:                "No rules triggered - no typologies",
			ruleResults:         []domain.RuleResult{},
			wantAccountTakeover: false,
			wantStructuring:     false,
		},
		{
			name: "Unknown rules - no impact",
			ruleResults: []domain.RuleResult{
				{RuleID: "unknown-rule", Score: 1.0},
			},
			wantAccountTakeover: false,
			wantStructuring:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := engine.EvaluateTypologies(tt.ruleResults)

			var accountTakeoverTriggered, structuringTriggered bool
			for _, r := range results {
				if r.TypologyID == "account-takeover" {
					accountTakeoverTriggered = r.Triggered
				}
				if r.TypologyID == "structuring" {
					structuringTriggered = r.Triggered
				}
			}

			if accountTakeoverTriggered != tt.wantAccountTakeover {
				t.Errorf("Account Takeover: got triggered=%v, want %v", accountTakeoverTriggered, tt.wantAccountTakeover)
			}
			if structuringTriggered != tt.wantStructuring {
				t.Errorf("Structuring: got triggered=%v, want %v", structuringTriggered, tt.wantStructuring)
			}
		})
	}
}

func TestTypologyEngine_GetTriggeredTypologies(t *testing.T) {
	engine := NewTypologyEngine()

	typologies := []*domain.Typology{
		{
			ID:             "typology-a",
			Name:           "Typology A",
			AlertThreshold: 0.5,
			Enabled:        true,
			Rules: []domain.TypologyRuleWeight{
				{RuleID: "rule-1", Weight: 1.0},
			},
		},
		{
			ID:             "typology-b",
			Name:           "Typology B",
			AlertThreshold: 0.8,
			Enabled:        true,
			Rules: []domain.TypologyRuleWeight{
				{RuleID: "rule-1", Weight: 1.0},
			},
		},
	}

	engine.LoadTypologies(typologies)

	ruleResults := []domain.RuleResult{
		{RuleID: "rule-1", Score: 0.6},
	}

	triggered := engine.GetTriggeredTypologies(ruleResults)

	if len(triggered) != 1 {
		t.Fatalf("Expected 1 triggered typology, got %d", len(triggered))
	}

	if triggered[0].TypologyID != "typology-a" {
		t.Errorf("Expected typology-a to trigger, got %s", triggered[0].TypologyID)
	}
}

func TestTypologyEngine_RuleContributions(t *testing.T) {
	engine := NewTypologyEngine()

	typologies := []*domain.Typology{
		{
			ID:             "test-typology",
			Name:           "Test Typology",
			AlertThreshold: 0.5,
			Enabled:        true,
			Rules: []domain.TypologyRuleWeight{
				{RuleID: "rule-1", Weight: 0.5},
				{RuleID: "rule-2", Weight: 0.3},
				{RuleID: "rule-3", Weight: 0.2},
			},
		},
	}

	engine.LoadTypologies(typologies)

	ruleResults := []domain.RuleResult{
		{RuleID: "rule-1", Score: 0.8},
		{RuleID: "rule-2", Score: 1.0},
		{RuleID: "rule-3", Score: 0.5},
	}

	results := engine.EvaluateTypologies(ruleResults)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]

	// Check score: 0.8*0.5 + 1.0*0.3 + 0.5*0.2 = 0.4 + 0.3 + 0.1 = 0.8
	expectedScore := 0.8
	// Use tolerance for floating point comparison
	if result.Score < expectedScore-0.001 || result.Score > expectedScore+0.001 {
		t.Errorf("Expected score ~%v, got %v", expectedScore, result.Score)
	}

	if len(result.Contributions) != 3 {
		t.Fatalf("Expected 3 contributions, got %d", len(result.Contributions))
	}

	// Verify contributions
	for _, c := range result.Contributions {
		switch c.RuleID {
		case "rule-1":
			if c.Contribution != 0.4 {
				t.Errorf("rule-1 contribution: expected 0.4, got %v", c.Contribution)
			}
		case "rule-2":
			if c.Contribution != 0.3 {
				t.Errorf("rule-2 contribution: expected 0.3, got %v", c.Contribution)
			}
		case "rule-3":
			if c.Contribution != 0.1 {
				t.Errorf("rule-3 contribution: expected 0.1, got %v", c.Contribution)
			}
		}
	}
}

func TestTypologyEngine_DisabledTypologies(t *testing.T) {
	engine := NewTypologyEngine()

	typologies := []*domain.Typology{
		{
			ID:             "enabled-typology",
			Name:           "Enabled",
			AlertThreshold: 0.5,
			Enabled:        true,
			Rules: []domain.TypologyRuleWeight{
				{RuleID: "rule-1", Weight: 1.0},
			},
		},
		{
			ID:             "disabled-typology",
			Name:           "Disabled",
			AlertThreshold: 0.5,
			Enabled:        false,
			Rules: []domain.TypologyRuleWeight{
				{RuleID: "rule-1", Weight: 1.0},
			},
		},
	}

	engine.LoadTypologies(typologies)

	if engine.TypologyCount() != 1 {
		t.Errorf("Expected 1 enabled typology, got %d", engine.TypologyCount())
	}

	loaded := engine.GetLoadedTypologies()
	if len(loaded) != 1 || loaded[0].ID != "enabled-typology" {
		t.Error("Only enabled typologies should be loaded")
	}
}

func TestTypologyEngine_ReloadTypologies(t *testing.T) {
	engine := NewTypologyEngine()

	// Initial load
	initial := []*domain.Typology{
		{ID: "typology-1", Name: "Typology 1", Enabled: true},
	}
	engine.LoadTypologies(initial)

	if engine.TypologyCount() != 1 {
		t.Errorf("Expected 1 typology after initial load, got %d", engine.TypologyCount())
	}

	// Reload with different typologies
	updated := []*domain.Typology{
		{ID: "typology-2", Name: "Typology 2", Enabled: true},
		{ID: "typology-3", Name: "Typology 3", Enabled: true},
	}
	engine.ReloadTypologies(updated)

	if engine.TypologyCount() != 2 {
		t.Errorf("Expected 2 typologies after reload, got %d", engine.TypologyCount())
	}

	// Verify old typology is gone
	_, exists := engine.EvaluateTypology("typology-1", nil)
	if exists {
		t.Error("typology-1 should not exist after reload")
	}
}

// TestTypologyEngine_ThresholdIgnoresFloatNoise covers weights 0.1 and 0.7,
// which sum to 0.7999999999999999 in float64 and must still trigger at 0.8.
func TestTypologyEngine_ThresholdIgnoresFloatNoise(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{{
		ID: "t", AlertThreshold: 0.8, Enabled: true,
		Rules: []domain.TypologyRuleWeight{{RuleID: "a", Weight: 0.1}, {RuleID: "b", Weight: 0.7}},
	}})
	res := engine.EvaluateTypologies([]domain.RuleResult{{RuleID: "a", Score: 1}, {RuleID: "b", Score: 1}})
	if len(res) != 1 || !res[0].Triggered || res[0].Score != 0.8 {
		t.Fatalf("got %+v, want triggered with score 0.8", res)
	}
}

// TestTypologyEngine_ZeroWeightIsNoContribution confirms the typology scoring
// path treats a weight of 0 as zero contribution (contribution = score * weight,
// no clamp). The TADP detection aggregator now honors the same semantics, so
// the rule and typology paths agree on what weight 0 means.
func TestTypologyEngine_ZeroWeightIsNoContribution(t *testing.T) {
	engine := NewTypologyEngine()

	typologies := []*domain.Typology{
		{
			ID:             "zero-weight-typology",
			Name:           "Zero Weight Typology",
			AlertThreshold: 0.5,
			Enabled:        true,
			Rules: []domain.TypologyRuleWeight{
				{RuleID: "contrib", Weight: 1.0},
				{RuleID: "monitor", Weight: 0.0},
			},
		},
	}
	engine.LoadTypologies(typologies)

	ruleResults := []domain.RuleResult{
		{RuleID: "contrib", Score: 0.4},
		{RuleID: "monitor", Score: 1.0},
	}

	results := engine.EvaluateTypologies(ruleResults)
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	result := results[0]

	// 0.4*1.0 + 1.0*0.0 = 0.4. The high monitor score must not raise it.
	if result.Score < 0.399 || result.Score > 0.401 {
		t.Errorf("Expected typology score ~0.4 (zero-weight rule excluded), got %v", result.Score)
	}
	if result.Triggered {
		t.Error("Expected typology NOT triggered (0.4 < 0.5 threshold)")
	}

	var monitorContrib *domain.RuleContribution
	for i := range result.Contributions {
		if result.Contributions[i].RuleID == "monitor" {
			monitorContrib = &result.Contributions[i]
		}
	}
	if monitorContrib == nil {
		t.Fatal("expected a contribution entry for the zero-weight rule")
	}
	if monitorContrib.Weight != 0.0 {
		t.Errorf("expected monitor weight 0, got %v", monitorContrib.Weight)
	}
	if monitorContrib.Contribution != 0.0 {
		t.Errorf("expected monitor contribution 0 (score * weight 0), got %v", monitorContrib.Contribution)
	}
}

// TestTypologyEngine_CompositeScoreCappedAtOne is the regression test for the
// out-of-range compliance-mode score. The shipped mule-account typology has
// rule weights 0.5 + 0.25 + 0.25 + 0.25 + 0.25 = 1.5. When every rule fires at
// score 1.0, the raw weighted sum is 1.5. Before the fix, result.Score was
// 1.5, which violates the documented 0.0-1.0 range. Now the sum is capped at
// 1.0. The per-rule contributions keep the raw (uncapped) values so the
// breakdown still shows what each rule added.
func TestTypologyEngine_CompositeScoreCappedAtOne(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{{
		ID:             "typology-mule-account",
		Name:           "Mule Account Activity",
		AlertThreshold: 0.75,
		Enabled:        true,
		Rules: []domain.TypologyRuleWeight{
			{RuleID: "pass-through-001", Weight: 0.5},
			{RuleID: "velocity-001", Weight: 0.25},
			{RuleID: "fan-out-001", Weight: 0.25},
			{RuleID: "account-drain-001", Weight: 0.25},
			{RuleID: "partial-drain-001", Weight: 0.25},
		},
	}})

	ruleResults := []domain.RuleResult{
		{RuleID: "pass-through-001", Score: 1.0},
		{RuleID: "velocity-001", Score: 1.0},
		{RuleID: "fan-out-001", Score: 1.0},
		{RuleID: "account-drain-001", Score: 1.0},
		{RuleID: "partial-drain-001", Score: 1.0},
	}

	results := engine.EvaluateTypologies(ruleResults)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Raw sum is 1.5. The score must be capped at 1.0, not 1.5.
	if results[0].Score != 1.0 {
		t.Errorf("expected capped score 1.0, got %v (raw weighted sum was 1.5)", results[0].Score)
	}
	// Any score at or above the threshold still triggers. The cap must not
	// change the alert decision.
	if !results[0].Triggered {
		t.Error("expected the typology to trigger (capped 1.0 >= threshold 0.75)")
	}

	// Contributions stay raw and honest. Their sum is 1.5, the uncapped
	// weighted sum, while the aggregate score is capped. This keeps the
	// per-rule breakdown visible to reviewers.
	var contribSum float64
	for _, c := range results[0].Contributions {
		contribSum += c.Contribution
	}
	if contribSum < 1.499 || contribSum > 1.501 {
		t.Errorf("expected raw contributions to still sum to 1.5, got %v", contribSum)
	}
}

// TestTypologyEngine_ScoreCapDoesNotSuppressSubOneScores is a regression guard
// that the cap only clamps the top. A score below 1.0 must pass through
// unchanged, including the float-noise case just under the threshold.
func TestTypologyEngine_ScoreCapDoesNotSuppressSubOneScores(t *testing.T) {
	engine := NewTypologyEngine()
	engine.LoadTypologies([]*domain.Typology{{
		ID: "below-one", AlertThreshold: 0.9, Enabled: true,
		Rules: []domain.TypologyRuleWeight{
			{RuleID: "a", Weight: 0.5},
			{RuleID: "b", Weight: 0.25},
		},
	}})

	// 0.8*0.5 + 1.0*0.25 = 0.65. Below 1.0, so the cap is a no-op.
	res := engine.EvaluateTypologies([]domain.RuleResult{
		{RuleID: "a", Score: 0.8},
		{RuleID: "b", Score: 1.0},
	})
	if len(res) != 1 || res[0].Score != 0.65 {
		t.Fatalf("expected score 0.65 (cap must not change sub-1.0 sums), got %+v", res)
	}
	if res[0].Triggered {
		t.Error("expected NOT triggered (0.65 < 0.9)")
	}
}

// TestTypologyEngine_SeedTypologiesScoreBounded loads the shipped seed config
// and asserts every typology score stays in the documented 0.0-1.0 range when
// every referenced rule fires at score 1.0 (the strictest case). This guards
// the actual shipped data: commit 669d572 raised four typology weight sums to
// 1.125-1.5, so without the cap this test reports 1.125 and 1.5.
func TestTypologyEngine_SeedTypologiesScoreBounded(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "configs", "typologies", "fatf-typologies.json"))
	if err != nil {
		t.Fatalf("read seed typologies: %v", err)
	}

	var file struct {
		Typologies []domain.Typology `json:"typologies"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("unmarshal seed typologies: %v", err)
	}

	enabled := make([]*domain.Typology, 0, len(file.Typologies))
	ruleIDs := make(map[string]struct{})
	for i := range file.Typologies {
		if !file.Typologies[i].Enabled {
			continue
		}
		enabled = append(enabled, &file.Typologies[i])
		for _, r := range file.Typologies[i].Rules {
			ruleIDs[r.RuleID] = struct{}{}
		}
	}
	if len(enabled) == 0 {
		t.Fatal("seed file has no enabled typologies; the test is no longer meaningful")
	}

	// Evaluate every referenced rule at score 1.0, the case that maximizes
	// the weighted sum and so maximizes the chance of exceeding 1.0.
	ruleResults := make([]domain.RuleResult, 0, len(ruleIDs))
	for id := range ruleIDs {
		ruleResults = append(ruleResults, domain.RuleResult{RuleID: id, Score: 1.0})
	}

	engine := NewTypologyEngine()
	engine.LoadTypologies(enabled)
	results := engine.EvaluateTypologies(ruleResults)

	for _, r := range results {
		if r.Score < 0.0 || r.Score > 1.0 {
			t.Errorf("typology %s: score %v is outside the documented 0.0-1.0 range",
				r.TypologyID, r.Score)
		}
	}
}
