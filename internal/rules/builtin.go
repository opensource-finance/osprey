package rules

import "github.com/opensource-finance/osprey/internal/domain"

// BuiltinRules returns an empty slice - all rules must be configured via database.
// This function exists for backward compatibility with tests.
func BuiltinRules() []*domain.RuleConfig {
	return []*domain.RuleConfig{}
}
