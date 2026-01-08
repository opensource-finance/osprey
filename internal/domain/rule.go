package domain

// RuleConfig defines a fraud detection rule configuration.
type RuleConfig struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`

	// CEL expression to evaluate
	Expression string `json:"expression"`

	// Outcome bands for score-to-decision mapping
	Bands []RuleBand `json:"bands"`

	// Rule weight in typology calculation
	Weight float64 `json:"weight"`

	// Whether rule is active
	Enabled bool `json:"enabled"`
}

// RuleBand maps a score range to an outcome.
type RuleBand struct {
	LowerLimit *float64 `json:"lowerLimit,omitempty"`
	UpperLimit *float64 `json:"upperLimit,omitempty"`
	SubRuleRef string   `json:"subRuleRef"` // e.g., ".pass", ".fail", ".review"
	Reason     string   `json:"reason"`
}

// RuleResult is the output of a rule evaluation.
type RuleResult struct {
	RuleID     string  `json:"ruleId"`
	TenantID   string  `json:"tenantId"`
	TxID       string  `json:"txId"`
	SubRuleRef string  `json:"subRuleRef"` // ".pass", ".fail", ".err"
	Score      float64 `json:"score"`      // The computed value
	Reason     string  `json:"reason"`
	Weight     float64 `json:"weight"`
	ProcessMs  int64   `json:"processMs"` // Processing time in milliseconds
}

// Predefined rule outcomes
const (
	RuleOutcomePass   = ".pass"
	RuleOutcomeFail   = ".fail"
	RuleOutcomeReview = ".review"
	RuleOutcomeError  = ".err"
)

// VelocityRule is a built-in rule for transaction velocity checks.
// Expression: transactions_count > threshold within time_window
type VelocityRule struct {
	EntityField string `json:"entityField"` // "debtorId" or "creditorId"
	Threshold   int    `json:"threshold"`   // Max transactions allowed
	WindowSecs  int    `json:"windowSecs"`  // Time window in seconds
}
