// Package tadp implements the Transaction Aggregated Decision Processor.
// TADP aggregates rule and typology results to make a final decision.
package tadp

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/opensource-finance/osprey/internal/domain"
)

// Processor aggregates rule results and produces a final decision.
type Processor struct {
	// Threshold above which a transaction is flagged as ALERT
	AlertThreshold float64

	// Weight configuration for rule aggregation
	UseWeightedScoring bool

	// Scoring selects how Detection mode combines rule scores:
	// - ScoringSum (default, also ""): sum of score x weight, capped at 1.
	//   A rule's weight is its contribution, whatever other rules exist.
	// - ScoringAverage: weighted average over all rules (before v0.2). Each new
	//   rule dilutes the others, including rules that did not match.
	Scoring string

	// Mode determines evaluation strategy:
	// - "detection": Rules → Weighted Score → Alert (fast, no typologies)
	// - "compliance": Rules → Typologies → FATF patterns (requires typologies)
	Mode string
}

// Scoring methods for Detection mode.
const (
	ScoringSum     = "sum"
	ScoringAverage = "average"
)

// NewProcessor creates a new TADP processor with default settings.
// Defaults to Detection mode - fast, weighted rule scoring.
func NewProcessor() *Processor {
	return &Processor{
		AlertThreshold:     0.7,         // Default threshold
		UseWeightedScoring: true,        // Use rule weights in scoring
		Scoring:            ScoringSum,  // Weights add up; no dilution
		Mode:               "detection", // Default: fast fraud detection
	}
}

// NewComplianceProcessor creates a processor for Compliance mode.
// Requires typologies for FATF-inspired evaluation.
func NewComplianceProcessor() *Processor {
	return &Processor{
		AlertThreshold:     0.7,
		UseWeightedScoring: true,
		Scoring:            ScoringSum,
		Mode:               "compliance",
	}
}

// NoTypologiesID marks a compliance evaluation that ran with no typologies
// loaded. The decision fails closed to ALRT.
const NoTypologiesID = "compliance-no-typologies"

// DecisionInput contains all data needed for a decision.
type DecisionInput struct {
	TenantID        string
	TxID            string
	TraceID         string
	RuleResults     []domain.RuleResult
	TypologyResults []domain.TypologyResult // From TypologyEngine evaluation
	StartTime       time.Time
}

// Process evaluates rule results and produces a final decision.
func (p *Processor) Process(ctx context.Context, input *DecisionInput) *domain.Evaluation {
	start := time.Now()

	eval := &domain.Evaluation{
		ID:          uuid.New().String(),
		TenantID:    input.TenantID,
		TxID:        input.TxID,
		Timestamp:   time.Now().UTC(),
		RuleResults: input.RuleResults,
	}

	// Aggregate rule results
	aggResult := p.aggregate(input.RuleResults)

	// Compliance Mode: Use typology results for FATF-inspired evaluation
	if p.Mode == "compliance" && len(input.TypologyResults) == 0 {
		// The handler and worker reject compliance requests when no typology is
		// loaded, but a concurrent delete or disable of the last typology can
		// empty the set after that check. Detection scoring is not a compliance
		// decision, so fail closed: alert for review and mark the evaluation.
		slog.Warn("compliance evaluation without typologies, failing closed to ALRT",
			"tenant_id", input.TenantID,
			"tx_id", input.TxID,
			"trace_id", input.TraceID,
		)
		eval.Status = domain.StatusAlert
		eval.Score = aggResult.AggregateScore
		eval.TypologyResults = []domain.TypologyResult{{
			TypologyID:   NoTypologiesID,
			TypologyName: "Compliance evaluation without typologies",
			Score:        aggResult.AggregateScore,
			Triggered:    true,
			Rules:        input.RuleResults,
		}}
	} else if p.Mode == "compliance" {
		eval.TypologyResults = input.TypologyResults

		// Check if any typology triggered
		anyTypologyTriggered := false
		maxTypologyScore := 0.0
		for _, t := range input.TypologyResults {
			if t.Triggered {
				anyTypologyTriggered = true
			}
			if t.Score > maxTypologyScore {
				maxTypologyScore = t.Score
			}
		}

		// Decision based on typology results
		if anyTypologyTriggered || aggResult.HasCriticalFailure {
			eval.Status = domain.StatusAlert
		} else {
			eval.Status = domain.StatusNoAlert
		}

		// Use highest typology score as the evaluation score
		eval.Score = maxTypologyScore
	} else {
		// Detection Mode: Fast, weighted rule aggregation (default)
		// No typologies required - direct score-to-alert decision
		if aggResult.HasCriticalFailure || aggResult.AggregateScore >= p.AlertThreshold {
			eval.Status = domain.StatusAlert
		} else {
			eval.Status = domain.StatusNoAlert
		}

		eval.Score = aggResult.AggregateScore

		// Build detection summary (optional typology-like grouping for reporting)
		eval.TypologyResults = p.buildDetectionSummary(input.RuleResults, aggResult)
	}

	// Populate metadata
	decisionMs := time.Since(start).Milliseconds()
	totalMs := time.Since(input.StartTime).Milliseconds()

	eval.Metadata = domain.EvaluationMetadata{
		TraceID:             input.TraceID,
		RulesEvaluated:      len(input.RuleResults),
		TypologiesEvaluated: len(input.TypologyResults),
		DecisionMs:          decisionMs,
		TotalMs:             totalMs,
		EngineVersion:       "osprey-1.0",
	}

	return eval
}

// AggregateResult holds the aggregated scoring results.
type AggregateResult struct {
	AggregateScore     float64
	TotalWeight        float64
	RulesTriggered     int
	HasCriticalFailure bool
}

// aggregate computes the weighted aggregate score from rule results.
func (p *Processor) aggregate(results []domain.RuleResult) *AggregateResult {
	if len(results) == 0 {
		return &AggregateResult{}
	}

	agg := &AggregateResult{}

	for _, r := range results {
		weight := r.Weight
		if weight <= 0 {
			weight = 1.0
		}

		// .fail and .err (rule failed to evaluate) force ALRT.
		switch r.SubRuleRef {
		case domain.RuleOutcomeFail:
			agg.HasCriticalFailure = true
			agg.RulesTriggered++
		case domain.RuleOutcomeError:
			agg.HasCriticalFailure = true
		case domain.RuleOutcomeReview:
			agg.RulesTriggered++
		}

		if p.UseWeightedScoring {
			agg.AggregateScore += r.Score * weight
			agg.TotalWeight += weight
		} else {
			agg.AggregateScore += r.Score
			agg.TotalWeight += 1.0
		}
	}

	if p.Scoring == ScoringAverage {
		if agg.TotalWeight > 0 {
			agg.AggregateScore = agg.AggregateScore / agg.TotalWeight
		}
	} else {
		agg.AggregateScore = min(agg.AggregateScore, 1.0)
	}
	agg.AggregateScore = domain.RoundScore(agg.AggregateScore)

	return agg
}

// buildDetectionSummary creates a summary for Detection mode.
// Groups all rules into a single "detection" result for consistent API response.
func (p *Processor) buildDetectionSummary(rules []domain.RuleResult, agg *AggregateResult) []domain.TypologyResult {
	if len(rules) == 0 {
		return nil
	}

	return []domain.TypologyResult{
		{
			TypologyID:   "detection-summary",
			TypologyName: "Detection Mode Summary",
			Score:        agg.AggregateScore,
			Threshold:    p.AlertThreshold,
			Triggered:    agg.AggregateScore >= p.AlertThreshold || agg.HasCriticalFailure,
			Rules:        rules,
		},
	}
}

// ShouldAlert returns true if the evaluation should trigger an alert.
func ShouldAlert(eval *domain.Evaluation) bool {
	return eval.Status == domain.StatusAlert
}

// GetReasons extracts the fail, review and error reasons from an evaluation.
func GetReasons(eval *domain.Evaluation) []string {
	var reasons []string
	for _, r := range eval.RuleResults {
		if r.SubRuleRef == domain.RuleOutcomeFail ||
			r.SubRuleRef == domain.RuleOutcomeReview ||
			r.SubRuleRef == domain.RuleOutcomeError {
			if r.Reason != "" {
				reasons = append(reasons, r.Reason)
			}
		}
	}
	return reasons
}
