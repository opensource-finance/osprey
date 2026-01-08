package rules

import (
	"testing"
)

func TestTypologyRulesEmpty(t *testing.T) {
	// TypologyRules now returns empty - all rules from database
	rules := TypologyRules()

	if len(rules) != 0 {
		t.Errorf("expected 0 typology rules (database-driven), got %d", len(rules))
	}
}

func TestAllRulesEmpty(t *testing.T) {
	// AllRules now returns empty - all rules from database
	rules := AllRules()

	if len(rules) != 0 {
		t.Errorf("expected 0 rules (database-driven), got %d", len(rules))
	}
}
