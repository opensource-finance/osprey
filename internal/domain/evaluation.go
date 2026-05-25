package domain

import (
	"time"
)

// Evaluation represents the complete evaluation result for a transaction.
type Evaluation struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	TxID      string    `json:"txId"`
	Status    string    `json:"status"` // "ALRT" or "NALT"
	Score     float64   `json:"score"`
	Timestamp time.Time `json:"timestamp"`

	// Rule results
	RuleResults []RuleResult `json:"ruleResults"`

	// Typology results (if applicable)
	TypologyResults []TypologyResult `json:"typologyResults,omitempty"`

	// Processing metadata
	Metadata EvaluationMetadata `json:"metadata"`
}

// TypologyResult is the aggregated result of rules for a typology.
type TypologyResult struct {
	TypologyID    string             `json:"typologyId"`
	TypologyName  string             `json:"typologyName"`
	Score         float64            `json:"score"`
	Threshold     float64            `json:"threshold"`
	Triggered     bool               `json:"triggered"`
	Rules         []RuleResult       `json:"rules"`
	Contributions []RuleContribution `json:"contributions,omitempty"`
	ProcessMs     int64              `json:"processMs,omitempty"`
}

// EvaluationMetadata contains processing information.
type EvaluationMetadata struct {
	TraceID             string `json:"traceId"`
	IngestMs            int64  `json:"ingestMs"`
	RulesMs             int64  `json:"rulesMs"`
	DecisionMs          int64  `json:"decisionMs"`
	TotalMs             int64  `json:"totalMs"`
	RulesEvaluated      int    `json:"rulesEvaluated"`
	TypologiesEvaluated int    `json:"typologiesEvaluated"`
	EngineVersion       string `json:"engineVersion"`
}

// Decision status constants
const (
	StatusAlert   = "ALRT" // Alert - suspicious transaction
	StatusNoAlert = "NALT" // No alert - transaction passed
)
