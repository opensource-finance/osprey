package domain

import "time"

// Typology defines a fraud detection typology configuration.
// A typology groups multiple rules with weights to calculate composite risk scores.
// Example: "Account Takeover" typology combines AccountDrain (0.4) + HighValue (0.25) + RapidMovement (0.2) + TxTypeRisk (0.15)
type Typology struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantId,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`

	// Rules contains the list of rules with their weights
	Rules []TypologyRuleWeight `json:"rules"`

	// AlertThreshold is the minimum score to trigger an alert (0.0-1.0)
	AlertThreshold float64 `json:"alertThreshold"`

	// Whether typology is active
	Enabled bool `json:"enabled"`

	// Audit timestamps
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

// TypologyRuleWeight defines a rule and its weight within a typology.
type TypologyRuleWeight struct {
	RuleID string  `json:"ruleId"`
	Weight float64 `json:"weight"` // 0.0 to 1.0
}

// RuleContribution shows how a single rule contributed to a typology score.
type RuleContribution struct {
	RuleID       string  `json:"ruleId"`
	RuleScore    float64 `json:"ruleScore"`    // Original rule score (0.0-1.0)
	Weight       float64 `json:"weight"`       // Weight in typology
	Contribution float64 `json:"contribution"` // ruleScore * weight
}

// Predefined typology IDs for default typologies
const (
	TypologyAccountTakeover = "typology-account-takeover"
	TypologyStructuring     = "typology-structuring"
	TypologyMuleAccount     = "typology-mule-account"
)
