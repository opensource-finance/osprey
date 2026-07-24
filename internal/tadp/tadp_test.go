package tadp

import (
	"context"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestProcessor(t *testing.T) {
	proc := NewProcessor()
	ctx := context.Background()

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

		eval := proc.Process(ctx, input)

		// Weighted: (1.0*1.0 + 0.1*5.0) / (1.0 + 5.0) = 1.5/6 = 0.25
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

		if eval.Status != domain.StatusNoAlert {
			t.Errorf("expected NALT for empty results, got %s", eval.Status)
		}
		if eval.Score != 0 {
			t.Errorf("expected score 0, got %.2f", eval.Score)
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
			{SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
		},
	}

	reasons := GetReasons(eval)

	if len(reasons) != 2 {
		t.Fatalf("expected 2 reasons, got %d", len(reasons))
	}

	if reasons[0] != "Velocity exceeded" {
		t.Errorf("expected 'Velocity exceeded', got '%s'", reasons[0])
	}
	if reasons[1] != "High value" {
		t.Errorf("expected 'High value', got '%s'", reasons[1])
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
