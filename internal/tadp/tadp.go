// Package tadp implements the Transaction Aggregated Decision Processor.
// TADP aggregates rule and typology results to make a final decision.
package tadp

import (
	"context"
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
}

// NewProcessor creates a new TADP processor with default settings.
func NewProcessor() *Processor {
	return &Processor{
		AlertThreshold:     0.7,  // Default threshold
		UseWeightedScoring: true, // Use rule weights in scoring
	}
}

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

	// Use typology results if provided by TypologyEngine
	if len(input.TypologyResults) > 0 {
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
		// Fallback: legacy behavior - single aggregated score
		if aggResult.HasCriticalFailure || aggResult.AggregateScore >= p.AlertThreshold {
			eval.Status = domain.StatusAlert
		} else {
			eval.Status = domain.StatusNoAlert
		}

		eval.Score = aggResult.AggregateScore

		// Build legacy typology results
		eval.TypologyResults = p.buildTypologyResults(input.RuleResults, aggResult)
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

		// Check for critical failures
		if r.SubRuleRef == domain.RuleOutcomeFail {
			agg.HasCriticalFailure = true
			agg.RulesTriggered++
		} else if r.SubRuleRef == domain.RuleOutcomeReview {
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

	// Normalize score
	if agg.TotalWeight > 0 {
		agg.AggregateScore = agg.AggregateScore / agg.TotalWeight
	}

	return agg
}

// buildTypologyResults groups rules into typology results.
// For now, creates a single "fraud-detection" typology.
func (p *Processor) buildTypologyResults(rules []domain.RuleResult, agg *AggregateResult) []domain.TypologyResult {
	if len(rules) == 0 {
		return nil
	}

	return []domain.TypologyResult{
		{
			TypologyID: "fraud-detection-001",
			Score:      agg.AggregateScore,
			Threshold:  p.AlertThreshold,
			Triggered:  agg.AggregateScore >= p.AlertThreshold || agg.HasCriticalFailure,
			Rules:      rules,
		},
	}
}

// ShouldAlert returns true if the evaluation should trigger an alert.
func ShouldAlert(eval *domain.Evaluation) bool {
	return eval.Status == domain.StatusAlert
}

// GetReasons extracts human-readable reasons from an evaluation.
func GetReasons(eval *domain.Evaluation) []string {
	var reasons []string
	for _, r := range eval.RuleResults {
		if r.SubRuleRef == domain.RuleOutcomeFail || r.SubRuleRef == domain.RuleOutcomeReview {
			if r.Reason != "" {
				reasons = append(reasons, r.Reason)
			}
		}
	}
	return reasons
}
