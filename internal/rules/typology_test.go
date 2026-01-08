package rules

import (
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
		name               string
		ruleResults        []domain.RuleResult
		wantAccountTakeover bool
		wantStructuring    bool
	}{
		{
			name: "Account takeover triggers - all rules fire",
			ruleResults: []domain.RuleResult{
				{RuleID: "account-drain-001", Score: 1.0},  // 0.4
				{RuleID: "high-value-001", Score: 1.0},    // 0.25
				{RuleID: "rapid-movement-001", Score: 1.0}, // 0.2
				{RuleID: "tx-type-risk-001", Score: 0.3},  // 0.045
			},
			wantAccountTakeover: true, // 0.4 + 0.25 + 0.2 + 0.045 = 0.895 >= 0.6
			wantStructuring:    false,
		},
		{
			name: "Account takeover triggers - partial rules",
			ruleResults: []domain.RuleResult{
				{RuleID: "account-drain-001", Score: 1.0}, // 0.4
				{RuleID: "high-value-001", Score: 1.0},   // 0.25
			},
			wantAccountTakeover: true, // 0.4 + 0.25 = 0.65 >= 0.6
			wantStructuring:    false,
		},
		{
			name: "Account takeover does NOT trigger - below threshold",
			ruleResults: []domain.RuleResult{
				{RuleID: "account-drain-001", Score: 0.5}, // 0.2
				{RuleID: "high-value-001", Score: 1.0},   // 0.25
			},
			wantAccountTakeover: false, // 0.2 + 0.25 = 0.45 < 0.6
			wantStructuring:    false,
		},
		{
			name: "Structuring triggers",
			ruleResults: []domain.RuleResult{
				{RuleID: "structuring-001", Score: 0.9},   // 0.45
				{RuleID: "round-amount-001", Score: 0.3}, // 0.09
			},
			wantAccountTakeover: false,
			wantStructuring:    true, // 0.45 + 0.09 = 0.54 >= 0.5
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
			wantStructuring:    true,
		},
		{
			name:               "No rules triggered - no typologies",
			ruleResults:        []domain.RuleResult{},
			wantAccountTakeover: false,
			wantStructuring:    false,
		},
		{
			name: "Unknown rules - no impact",
			ruleResults: []domain.RuleResult{
				{RuleID: "unknown-rule", Score: 1.0},
			},
			wantAccountTakeover: false,
			wantStructuring:    false,
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
