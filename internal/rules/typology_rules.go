package rules

import "github.com/opensource-finance/osprey/internal/domain"

// TypologyRules returns an empty slice - all rules must be configured via database.
// This function exists for backward compatibility with tests.
func TypologyRules() []*domain.RuleConfig {
	return []*domain.RuleConfig{}
}

// AllRules returns an empty slice - all rules must be configured via database.
// Rules should be created via POST /rules API and loaded from database on startup.
func AllRules() []*domain.RuleConfig {
	return []*domain.RuleConfig{}
}
