package tadp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestProcessor(t *testing.T) {
	proc := NewProcessor()
	ctx := context.Background()

	t.Run("EvalErrorForcesAlert", func(t *testing.T) {
		eval := proc.Process(ctx, &DecisionInput{
			TenantID:  "tenant-001",
			TxID:      "tx-err",
			StartTime: time.Now(),
			RuleResults: []domain.RuleResult{
				{RuleID: "broken", Score: 0, SubRuleRef: domain.RuleOutcomeError, Weight: 1.0},
				{RuleID: "ok", Score: 0.1, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
			},
		})
		if eval.Status != domain.StatusAlert {
			t.Errorf("a rule eval error must fail secure to ALRT, got %s", eval.Status)
		}
	})

	t.Run("AllPass", func(t *testing.T) {
		input := &DecisionInput{
			TenantID:  "tenant-001",
			TxID:      "tx-001",
			TraceID:   "trace-001",
			StartTime: time.Now(),
			RuleResults: []domain.RuleResult{
				{RuleID: "rule-1", Score: 0.1, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
				{RuleID: "rule-2", Score: 0.2, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
				{RuleID: "rule-3", Score: 0.1, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
			},
		}

		eval := proc.Process(ctx, input)

		if eval.Status != domain.StatusNoAlert {
			t.Errorf("expected NALT, got %s", eval.Status)
		}
		if eval.Score > proc.AlertThreshold {
			t.Errorf("score %.2f should be below threshold %.2f", eval.Score, proc.AlertThreshold)
		}
		if eval.TenantID != "tenant-001" {
			t.Errorf("expected tenantID 'tenant-001', got '%s'", eval.TenantID)
		}
		if eval.Metadata.TraceID != "trace-001" {
			t.Errorf("expected traceID 'trace-001', got '%s'", eval.Metadata.TraceID)
		}
	})

	t.Run("CriticalFailure", func(t *testing.T) {
		input := &DecisionInput{
			TenantID:  "tenant-001",
			TxID:      "tx-002",
			TraceID:   "trace-002",
			StartTime: time.Now(),
			RuleResults: []domain.RuleResult{
				{RuleID: "rule-1", Score: 0.1, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
				{RuleID: "rule-2", Score: 1.0, SubRuleRef: domain.RuleOutcomeFail, Weight: 1.0}, // Fail
				{RuleID: "rule-3", Score: 0.1, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
			},
		}

		eval := proc.Process(ctx, input)

		if eval.Status != domain.StatusAlert {
			t.Errorf("expected ALRT for critical failure, got %s", eval.Status)
		}
	})

	t.Run("HighAggregateScore", func(t *testing.T) {
		input := &DecisionInput{
			TenantID:  "tenant-001",
			TxID:      "tx-003",
			TraceID:   "trace-003",
			StartTime: time.Now(),
			RuleResults: []domain.RuleResult{
				{RuleID: "rule-1", Score: 0.8, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
				{RuleID: "rule-2", Score: 0.9, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
				{RuleID: "rule-3", Score: 0.7, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
			},
		}

		eval := proc.Process(ctx, input)

		// Average is 0.8, which is above 0.7 threshold
		if eval.Status != domain.StatusAlert {
			t.Errorf("expected ALRT for high score, got %s", eval.Status)
		}
	})

	t.Run("WeightedScoring", func(t *testing.T) {
		input := &DecisionInput{
			TenantID:  "tenant-001",
			TxID:      "tx-004",
			TraceID:   "trace-004",
			StartTime: time.Now(),
			RuleResults: []domain.RuleResult{
				{RuleID: "rule-1", Score: 1.0, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0}, // High score, low weight
				{RuleID: "rule-2", Score: 0.1, SubRuleRef: domain.RuleOutcomePass, Weight: 5.0},   // Low score, high weight
			},
		}

		avg := NewProcessor()
		avg.Scoring = ScoringAverage
		eval := avg.Process(ctx, input)

		// Weighted average: (1.0*1.0 + 0.1*5.0) / (1.0 + 5.0) = 1.5/6 = 0.25
		if eval.Score > 0.3 {
			t.Errorf("expected weighted score ~0.25, got %.2f", eval.Score)
		}
		if eval.Status != domain.StatusNoAlert {
			t.Errorf("expected NALT with weighted scoring, got %s", eval.Status)
		}
	})

	t.Run("EmptyResults", func(t *testing.T) {
		input := &DecisionInput{
			TenantID:    "tenant-001",
			TxID:        "tx-005",
			TraceID:     "trace-005",
			StartTime:   time.Now(),
			RuleResults: []domain.RuleResult{},
		}

		eval := proc.Process(ctx, input)

		// Empty rule results mean the engine evaluated zero rules. Fail
		// closed to ALRT with the no-rules marker, not a clean NALT pass.
		if eval.Status != domain.StatusAlert {
			t.Errorf("expected ALRT (fail closed) for empty results, got %s", eval.Status)
		}
		if len(eval.TypologyResults) != 1 || eval.TypologyResults[0].TypologyID != NoRulesID {
			t.Fatalf("expected one %q typology result, got %+v", NoRulesID, eval.TypologyResults)
		}
		if !eval.TypologyResults[0].Triggered {
			t.Error("expected the no-rules marker result to be triggered")
		}
		if eval.Score != 0 {
			t.Errorf("expected score 0, got %.2f", eval.Score)
		}
		if eval.Metadata.RulesEvaluated != 0 {
			t.Errorf("expected RulesEvaluated 0, got %d", eval.Metadata.RulesEvaluated)
		}
	})

	t.Run("MetadataPopulated", func(t *testing.T) {
		input := &DecisionInput{
			TenantID:  "tenant-001",
			TxID:      "tx-006",
			TraceID:   "trace-006",
			StartTime: time.Now(),
			RuleResults: []domain.RuleResult{
				{RuleID: "rule-1", Score: 0.1, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
				{RuleID: "rule-2", Score: 0.2, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
			},
		}

		eval := proc.Process(ctx, input)

		if eval.Metadata.TraceID != "trace-006" {
			t.Error("missing traceID in metadata")
		}
		if eval.Metadata.RulesEvaluated != 2 {
			t.Errorf("expected 2 rules evaluated, got %d", eval.Metadata.RulesEvaluated)
		}
		if eval.Metadata.EngineVersion == "" {
			t.Error("missing engine version")
		}
		if eval.Metadata.TotalMs < 0 {
			t.Error("TotalMs should be non-negative")
		}
	})

	t.Run("TypologyResults", func(t *testing.T) {
		input := &DecisionInput{
			TenantID:  "tenant-001",
			TxID:      "tx-007",
			TraceID:   "trace-007",
			StartTime: time.Now(),
			RuleResults: []domain.RuleResult{
				{RuleID: "rule-1", Score: 0.5, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
			},
		}

		eval := proc.Process(ctx, input)

		if len(eval.TypologyResults) != 1 {
			t.Fatalf("expected 1 typology result, got %d", len(eval.TypologyResults))
		}

		typo := eval.TypologyResults[0]
		if typo.TypologyID == "" {
			t.Error("missing typology ID")
		}
		if len(typo.Rules) != 1 {
			t.Errorf("expected 1 rule in typology, got %d", len(typo.Rules))
		}
	})
}

func TestShouldAlert(t *testing.T) {
	alertEval := &domain.Evaluation{Status: domain.StatusAlert}
	passEval := &domain.Evaluation{Status: domain.StatusNoAlert}

	if !ShouldAlert(alertEval) {
		t.Error("expected true for ALRT")
	}
	if ShouldAlert(passEval) {
		t.Error("expected false for NALT")
	}
}

func TestGetReasons(t *testing.T) {
	eval := &domain.Evaluation{
		RuleResults: []domain.RuleResult{
			{SubRuleRef: domain.RuleOutcomePass, Reason: "All good"},
			{SubRuleRef: domain.RuleOutcomeFail, Reason: "Velocity exceeded"},
			{SubRuleRef: domain.RuleOutcomeReview, Reason: "High value"},
			{SubRuleRef: domain.RuleOutcomeError, Reason: "evaluation error: no such key: ml_score"},
			{SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
		},
	}

	reasons := GetReasons(eval)

	if len(reasons) != 3 {
		t.Fatalf("expected 3 reasons (fail + review + err), got %d: %v", len(reasons), reasons)
	}

	if reasons[0] != "Velocity exceeded" {
		t.Errorf("expected 'Velocity exceeded', got '%s'", reasons[0])
	}
	if reasons[1] != "High value" {
		t.Errorf("expected 'High value', got '%s'", reasons[1])
	}
	if reasons[2] != "evaluation error: no such key: ml_score" {
		t.Errorf("expected errored rule reason surfaced, got '%s'", reasons[2])
	}
}

// TestGetReasonsSurfacesErrorOutcome checks a rule that errored at eval appears in the reasons.
func TestGetReasonsSurfacesErrorOutcome(t *testing.T) {
	eval := &domain.Evaluation{
		RuleResults: []domain.RuleResult{
			{SubRuleRef: domain.RuleOutcomeError, Reason: "evaluation error: division by zero"},
		},
	}

	reasons := GetReasons(eval)

	if len(reasons) != 1 {
		t.Fatalf("expected errored rule reason to be surfaced, got %d: %v", len(reasons), reasons)
	}
	if reasons[0] != "evaluation error: division by zero" {
		t.Errorf("expected evaluation error reason, got '%s'", reasons[0])
	}
}

func TestCustomThreshold(t *testing.T) {
	proc := &Processor{
		AlertThreshold:     0.5, // Lower threshold
		UseWeightedScoring: true,
	}

	ctx := context.Background()
	input := &DecisionInput{
		TenantID:  "tenant-001",
		TxID:      "tx-001",
		TraceID:   "trace-001",
		StartTime: time.Now(),
		RuleResults: []domain.RuleResult{
			{RuleID: "rule-1", Score: 0.6, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
		},
	}

	eval := proc.Process(ctx, input)

	// 0.6 > 0.5 threshold, should alert
	if eval.Status != domain.StatusAlert {
		t.Errorf("expected ALRT with 0.5 threshold, got %s", eval.Status)
	}
}

func TestUnweightedScoring(t *testing.T) {
	proc := &Processor{
		AlertThreshold:     0.7,
		UseWeightedScoring: false, // Disable weighted scoring
		Scoring:            ScoringAverage,
	}

	ctx := context.Background()
	input := &DecisionInput{
		TenantID:  "tenant-001",
		TxID:      "tx-001",
		TraceID:   "trace-001",
		StartTime: time.Now(),
		RuleResults: []domain.RuleResult{
			{RuleID: "rule-1", Score: 0.4, SubRuleRef: domain.RuleOutcomeReview, Weight: 10.0}, // Weight ignored
			{RuleID: "rule-2", Score: 0.4, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
		},
	}

	eval := proc.Process(ctx, input)

	// Unweighted: (0.4 + 0.4) / 2 = 0.4
	if eval.Score > 0.5 {
		t.Errorf("expected unweighted score ~0.4, got %.2f", eval.Score)
	}
}

// ============================================================================
// COMPLIANCE MODE TESTS
// ============================================================================

func TestNewComplianceProcessor(t *testing.T) {
	proc := NewComplianceProcessor()

	if proc.Mode != "compliance" {
		t.Errorf("expected mode 'compliance', got '%s'", proc.Mode)
	}
	if proc.AlertThreshold != 0.7 {
		t.Errorf("expected threshold 0.7, got %.2f", proc.AlertThreshold)
	}
	if !proc.UseWeightedScoring {
		t.Error("expected UseWeightedScoring to be true")
	}
}

func TestDetectionModeDefault(t *testing.T) {
	proc := NewProcessor()

	if proc.Mode != "detection" {
		t.Errorf("expected default mode 'detection', got '%s'", proc.Mode)
	}
}

func TestComplianceModeWithTypologies(t *testing.T) {
	proc := NewComplianceProcessor()
	ctx := context.Background()

	// Typology triggered (score above threshold)
	input := &DecisionInput{
		TenantID:  "tenant-001",
		TxID:      "tx-001",
		TraceID:   "trace-001",
		StartTime: time.Now(),
		RuleResults: []domain.RuleResult{
			{RuleID: "rule-1", Score: 0.8, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
		},
		TypologyResults: []domain.TypologyResult{
			{
				TypologyID:   "typo-structuring",
				TypologyName: "Structuring Detection",
				Score:        0.85,
				Threshold:    0.6,
				Triggered:    true, // Typology triggered
			},
		},
	}

	eval := proc.Process(ctx, input)

	if eval.Status != domain.StatusAlert {
		t.Errorf("expected ALRT when typology triggered in compliance mode, got %s", eval.Status)
	}
	if eval.Score != 0.85 {
		t.Errorf("expected score to be max typology score 0.85, got %.2f", eval.Score)
	}
	if len(eval.TypologyResults) != 1 {
		t.Errorf("expected 1 typology result, got %d", len(eval.TypologyResults))
	}
}

func TestComplianceModeNoTypologyTriggered(t *testing.T) {
	proc := NewComplianceProcessor()
	ctx := context.Background()

	// Typology NOT triggered (score below threshold)
	input := &DecisionInput{
		TenantID:  "tenant-001",
		TxID:      "tx-002",
		TraceID:   "trace-002",
		StartTime: time.Now(),
		RuleResults: []domain.RuleResult{
			{RuleID: "rule-1", Score: 0.3, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
		},
		TypologyResults: []domain.TypologyResult{
			{
				TypologyID:   "typo-structuring",
				TypologyName: "Structuring Detection",
				Score:        0.4,
				Threshold:    0.6,
				Triggered:    false, // Not triggered
			},
		},
	}

	eval := proc.Process(ctx, input)

	if eval.Status != domain.StatusNoAlert {
		t.Errorf("expected NALT when no typology triggered, got %s", eval.Status)
	}
}

func TestComplianceModeCriticalFailureOverridesTypology(t *testing.T) {
	proc := NewComplianceProcessor()
	ctx := context.Background()

	// Critical failure should trigger alert even if typology not triggered
	input := &DecisionInput{
		TenantID:  "tenant-001",
		TxID:      "tx-003",
		TraceID:   "trace-003",
		StartTime: time.Now(),
		RuleResults: []domain.RuleResult{
			{RuleID: "rule-1", Score: 1.0, SubRuleRef: domain.RuleOutcomeFail, Weight: 1.0}, // Critical fail
		},
		TypologyResults: []domain.TypologyResult{
			{
				TypologyID: "typo-1",
				Score:      0.3,
				Threshold:  0.6,
				Triggered:  false, // Typology not triggered
			},
		},
	}

	eval := proc.Process(ctx, input)

	if eval.Status != domain.StatusAlert {
		t.Errorf("expected ALRT on critical failure even in compliance mode, got %s", eval.Status)
	}
}

func TestDetectionModeIgnoresTypologyResults(t *testing.T) {
	proc := NewProcessor() // Detection mode
	ctx := context.Background()

	// Even if typology results are passed, detection mode should use weighted scoring
	input := &DecisionInput{
		TenantID:  "tenant-001",
		TxID:      "tx-001",
		TraceID:   "trace-001",
		StartTime: time.Now(),
		RuleResults: []domain.RuleResult{
			{RuleID: "rule-1", Score: 0.3, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
		},
		TypologyResults: []domain.TypologyResult{
			{
				TypologyID: "typo-1",
				Score:      0.9, // High typology score
				Threshold:  0.6,
				Triggered:  true, // Typology would trigger
			},
		},
	}

	eval := proc.Process(ctx, input)

	// Detection mode should use rule score (0.3), not typology score (0.9)
	if eval.Score > 0.5 {
		t.Errorf("detection mode should use rule score, not typology score; got %.2f", eval.Score)
	}
	if eval.Status != domain.StatusNoAlert {
		t.Errorf("detection mode should be NALT with low rule score, got %s", eval.Status)
	}
}

// TestComplianceModeWithoutTypologyResultsFailsClosed covers issue #54. A
// compliance evaluation with no typology results used to fall back to
// detection scoring. A low aggregate score then gave NALT, a missed alert.
func TestComplianceModeWithoutTypologyResultsFailsClosed(t *testing.T) {
	proc := NewComplianceProcessor()

	for name, typologies := range map[string][]domain.TypologyResult{
		"nil":   nil,
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			eval := proc.Process(context.Background(), &DecisionInput{
				TenantID:  "tenant-001",
				TxID:      "tx-054",
				StartTime: time.Now(),
				RuleResults: []domain.RuleResult{
					{RuleID: "rule-1", Score: 0.3, SubRuleRef: domain.RuleOutcomePass, Weight: 1.0},
				},
				TypologyResults: typologies,
			})

			if eval.Status != domain.StatusAlert {
				t.Errorf("expected ALRT (fail closed), got %s", eval.Status)
			}
			if len(eval.TypologyResults) != 1 || eval.TypologyResults[0].TypologyID != NoTypologiesID {
				t.Fatalf("expected one %q typology result, got %+v", NoTypologiesID, eval.TypologyResults)
			}
			if !eval.TypologyResults[0].Triggered {
				t.Error("expected the marker typology result to be triggered")
			}
			if eval.Metadata.TypologiesEvaluated != 0 {
				t.Errorf("expected TypologiesEvaluated 0, got %d", eval.Metadata.TypologiesEvaluated)
			}
		})
	}
}

// TestSumScoringDoesNotDilute checks that, with sum scoring, rules that do not
// match add nothing: a rule's weight is its contribution, whatever other rules exist.
func TestSumScoringDoesNotDilute(t *testing.T) {
	ctx := context.Background()
	results := func(matchedWeight float64, quiet int) []domain.RuleResult {
		rr := []domain.RuleResult{{RuleID: "signal", Score: 1, SubRuleRef: domain.RuleOutcomeReview, Weight: matchedWeight}}
		for i := range quiet {
			rr = append(rr, domain.RuleResult{RuleID: fmt.Sprintf("quiet-%d", i), Score: 0, SubRuleRef: domain.RuleOutcomePass, Weight: 1})
		}
		return rr
	}

	for _, quiet := range []int{0, 1, 20} {
		sum := NewProcessor()
		if got := sum.Process(ctx, &DecisionInput{RuleResults: results(0.8, quiet)}); got.Status != domain.StatusAlert || got.Score != 0.8 {
			t.Errorf("sum, %d quiet rules: want ALRT 0.8, got %s %.2f", quiet, got.Status, got.Score)
		}
		if got := sum.Process(ctx, &DecisionInput{RuleResults: results(0.6, quiet)}); got.Status != domain.StatusNoAlert || got.Score != 0.6 {
			t.Errorf("sum, %d quiet rules: want NALT 0.6, got %s %.2f", quiet, got.Status, got.Score)
		}
	}

	// The average dilutes: the same 0.8 signal next to 3 quiet rules scores 0.8/3.8, about 0.21.
	avg := NewProcessor()
	avg.Scoring = ScoringAverage
	if got := avg.Process(ctx, &DecisionInput{RuleResults: results(0.8, 3)}); got.Status != domain.StatusNoAlert || got.Score > 0.25 {
		t.Errorf("average: want NALT about 0.21, got %s %.2f", got.Status, got.Score)
	}
}

func TestSumScoringCapsAtOne(t *testing.T) {
	rr := []domain.RuleResult{
		{RuleID: "a", Score: 1, SubRuleRef: domain.RuleOutcomeReview, Weight: 0.9},
		{RuleID: "b", Score: 1, SubRuleRef: domain.RuleOutcomeReview, Weight: 0.9},
	}
	if got := NewProcessor().Process(context.Background(), &DecisionInput{RuleResults: rr}); got.Score != 1.0 {
		t.Errorf("want score capped at 1.0, got %.2f", got.Score)
	}
}

// TestSumScoringReachesThresholdInAnyOrder checks that weights which add up to
// the threshold on paper trigger an alert whatever order the rules finish in.
func TestSumScoringReachesThresholdInAnyOrder(t *testing.T) {
	p := NewProcessor()
	orders := [][]float64{{0.1, 0.2, 0.4}, {0.4, 0.2, 0.1}, {0.2, 0.1, 0.4}, {0.3, 0.4}, {0.4, 0.3}, {0.1, 0.6}}
	for _, weights := range orders {
		var results []domain.RuleResult
		for i, w := range weights {
			results = append(results, domain.RuleResult{RuleID: fmt.Sprintf("r%d", i), Score: 1, Weight: w, SubRuleRef: domain.RuleOutcomeReview})
		}
		eval := p.Process(context.Background(), &DecisionInput{RuleResults: results})
		if eval.Status != domain.StatusAlert || eval.Score != 0.7 {
			t.Errorf("weights %v: got %s score %v, want ALRT 0.7", weights, eval.Status, eval.Score)
		}
	}
}

// TestSumScoringBelowThresholdByFloatNoise covers 0.1+0.7, which is
// 0.7999999999999999 in float64 and must still reach a threshold of 0.8.
func TestSumScoringBelowThresholdByFloatNoise(t *testing.T) {
	p := NewProcessor()
	p.AlertThreshold = 0.8
	eval := p.Process(context.Background(), &DecisionInput{RuleResults: []domain.RuleResult{
		{RuleID: "a", Score: 1, Weight: 0.1, SubRuleRef: domain.RuleOutcomeReview},
		{RuleID: "b", Score: 1, Weight: 0.7, SubRuleRef: domain.RuleOutcomeReview},
	}})
	if eval.Status != domain.StatusAlert || eval.Score != 0.8 {
		t.Fatalf("got %s score %v, want ALRT 0.8", eval.Status, eval.Score)
	}
}

// TestEmptyRuleResultsFailsClosed covers the zero-rules fail-open bug. When
// the rule engine evaluates zero rules (empty RuleResults), both detection
// mode and compliance mode used to score the empty set to zero and return
// NALT. A deployment with an empty rule set silently stopped generating
// alerts. Now the processor fails closed to ALRT with the no-rules marker.
func TestEmptyRuleResultsFailsClosed(t *testing.T) {
	// One compliance sub-case passes real typology results. With zero rule
	// results the typology rule-score map is empty, every typology scores 0,
	// and compliance mode would otherwise return NALT. This is the case the
	// changelog calls out: compliance mode with typologies loaded still fails
	// open when the rule set is empty.
	typologies := []domain.TypologyResult{
		{
			TypologyID: "typo-structuring",
			Score:      0.0,
			Threshold:  0.4,
			Triggered:  false,
		},
	}

	procedures := map[string]*Processor{
		"detection":  NewProcessor(),
		"compliance": NewComplianceProcessor(),
	}

	for mode, proc := range procedures {
		t.Run(mode, func(t *testing.T) {
			for name, rules := range map[string][]domain.RuleResult{
				"nil":   nil,
				"empty": {},
			} {
				t.Run(name, func(t *testing.T) {
					input := &DecisionInput{
						TenantID:        "tenant-001",
						TxID:            "tx-no-rules",
						TraceID:         "trace-no-rules",
						StartTime:       time.Now(),
						RuleResults:     rules,
						TypologyResults: typologies,
					}

					eval := proc.Process(context.Background(), input)

					if eval.Status != domain.StatusAlert {
						t.Fatalf("expected ALRT (fail closed) for empty rule results, got %s", eval.Status)
					}
					if len(eval.TypologyResults) != 1 || eval.TypologyResults[0].TypologyID != NoRulesID {
						t.Fatalf("expected one %q typology result, got %+v", NoRulesID, eval.TypologyResults)
					}
					if !eval.TypologyResults[0].Triggered {
						t.Error("expected the no-rules marker result to be triggered")
					}
					if eval.Score != 0 {
						t.Errorf("expected aggregate score 0, got %.2f", eval.Score)
					}
					if eval.Metadata.RulesEvaluated != 0 {
						t.Errorf("expected RulesEvaluated 0, got %d", eval.Metadata.RulesEvaluated)
					}
				})
			}
		})
	}
}

// TestEmptyRuleResultsTakePrecedenceOverNoTypologies checks that when both
// the rule set and the typology set are empty, the no-rules marker wins. The
// empty rule set is the root cause: typology scoring cannot run without rule
// results, so the more specific compliance-no-typologies marker is not the
// right signal.
func TestEmptyRuleResultsTakePrecedenceOverNoTypologies(t *testing.T) {
	proc := NewComplianceProcessor()

	eval := proc.Process(context.Background(), &DecisionInput{
		TenantID:    "tenant-001",
		TxID:        "tx-both-empty",
		StartTime:   time.Now(),
		RuleResults: []domain.RuleResult{},
		// Nil typologies: the compliance-no-typologies branch would also
		// fire. The no-rules branch must take precedence.
		TypologyResults: nil,
	})

	if eval.Status != domain.StatusAlert {
		t.Fatalf("expected ALRT, got %s", eval.Status)
	}
	if len(eval.TypologyResults) != 1 || eval.TypologyResults[0].TypologyID != NoRulesID {
		t.Fatalf("expected the no-rules marker to win over %q, got %+v", NoTypologiesID, eval.TypologyResults)
	}
}
