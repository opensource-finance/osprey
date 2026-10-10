package tadp

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// captureSlog redirects the default slog logger to a buffer for the duration of
// a test, restoring the previous default on return. Tests in this package do not
// run in parallel, so mutating the global default is safe.
func captureSlog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	newLogger := slog.New(handler)
	prev := slog.Default()
	slog.SetDefault(newLogger)
	return &buf, func() { slog.SetDefault(prev) }
}

func TestComplianceModeEmptyTypologyResultsFailsClosedToAlert(t *testing.T) {
	proc := NewComplianceProcessor()
	proc.AlertThreshold = 0.7
	ctx := context.Background()

	// Aggregate score 0.2 < 0.7 -> detection mode would NALT this. Compliance
	// mode with no typology results must NOT fall through to detection scoring;
	// it must fail closed to ALRT so a regulated transaction is reviewed.
	input := &DecisionInput{
		TenantID:  "tenant-001",
		TxID:      "tx-failclosed",
		TraceID:   "trace-001",
		StartTime: time.Now(),
		RuleResults: []domain.RuleResult{
			{RuleID: "rule-a", Score: 1.0, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
			{RuleID: "rule-b", Score: 0.0, SubRuleRef: domain.RuleOutcomePass, Weight: 4.0},
		},
		// TypologyResults deliberately nil/empty.
	}

	eval := proc.Process(ctx, input)

	if eval.Status != domain.StatusAlert {
		t.Fatalf("compliance mode with empty TypologyResults must fail closed to ALRT, got %s", eval.Status)
	}
	// Metadata must honestly report zero typologies evaluated: the bug relied
	// on a synthetic "detection-summary" stamping TypologiesEvaluated=0 but
	// surfacing a detection-mode decision; the honest marker is still 0, but
	// the decision must now be ALRT (fail-secure), not a detection-mode NALT.
	if eval.Metadata.TypologiesEvaluated != 0 {
		t.Errorf("expected TypologiesEvaluated=0 (none evaluated), got %d", eval.Metadata.TypologiesEvaluated)
	}
}

func TestComplianceModeEmptyTypologyResultsDoesNotStampDetectionSummary(t *testing.T) {
	proc := NewComplianceProcessor()
	ctx := context.Background()

	input := &DecisionInput{
		TenantID:    "tenant-001",
		TxID:        "tx-no-summary",
		StartTime:   time.Now(),
		RuleResults: []domain.RuleResult{{RuleID: "rule-a", Score: 0.8, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0}},
	}

	eval := proc.Process(ctx, input)

	// The audit trail must not contain a synthetic "detection-summary" typology
	// that would mislabel this as a detection-mode decision.
	if len(eval.TypologyResults) != 0 {
		t.Fatalf("expected no TypologyResults for fail-closed compliance eval, got %d", len(eval.TypologyResults))
	}
	for _, tr := range eval.TypologyResults {
		if tr.TypologyID == "detection-summary" {
			t.Fatalf("fail-closed compliance eval must not stamp a detection-summary typology; found %q", tr.TypologyID)
		}
	}
}

func TestComplianceModeEmptyTypologyResultsEmitsWarning(t *testing.T) {
	proc := NewComplianceProcessor()
	ctx := context.Background()

	buf, restore := captureSlog(t)
	defer restore()

	input := &DecisionInput{
		TenantID:  "tenant-001",
		TxID:      "tx-warn",
		TraceID:   "trace-warn",
		StartTime: time.Now(),
		RuleResults: []domain.RuleResult{
			{RuleID: "rule-a", SubRuleRef: domain.RuleOutcomeError, Reason: "boom", Weight: 1.0},
		},
	}

	_ = proc.Process(ctx, input)

	out := buf.String()
	if !strings.Contains(out, "compliance mode evaluation received no typology results") {
		t.Fatalf("expected a warning that compliance mode had no typology results; got log:\n%s", out)
	}
	// The warning must not be silent about which transaction was affected.
	if !strings.Contains(out, "tx-warn") {
		t.Fatalf("expected warning to include tx_id=tx-warn; got log:\n%s", out)
	}
}

// TestComplianceModeFalseAlertDirection reproduces the false-alert direction
// from the bug report. With typology results the decision is NALT (typology
// score below threshold). Without typology results the OLD code produced a
// detection-mode ALRT (aggregate >= threshold); the fix produces a fail-closed
// ALRT instead. Both are ALRT without typologies, but the fix is ALRT for the
// right reason (fail-secure, no detection-summary), and the with-typology path
// is unaffected.
func TestComplianceModeFalseAlertDirection(t *testing.T) {
	proc := NewComplianceProcessor()
	proc.AlertThreshold = 0.7
	ctx := context.Background()

	rules := []domain.RuleResult{
		{RuleID: "rule-a", Score: 0.8, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
		{RuleID: "rule-b", Score: 0.8, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
	}
	// Weighted aggregate = (0.8*1.0 + 0.8*1.0) / 2.0 = 0.8 >= 0.7 -> detection ALRT.
	// Typology score = 0.8 * 1.0 = 0.8 < 0.9 -> not triggered -> compliance NALT.
	typologyResults := []domain.TypologyResult{
		{TypologyID: "structuring-check", Score: 0.8, Threshold: 0.9, Triggered: false},
	}

	withTypo := proc.Process(ctx, &DecisionInput{
		TenantID:        "t",
		TxID:            "with",
		StartTime:       time.Now(),
		RuleResults:     rules,
		TypologyResults: typologyResults,
	})
	if withTypo.Status != domain.StatusNoAlert {
		t.Fatalf("with typology results: expected NALT, got %s", withTypo.Status)
	}

	withoutTypo := proc.Process(ctx, &DecisionInput{
		TenantID:    "t",
		TxID:        "without",
		StartTime:   time.Now(),
		RuleResults: rules,
		// TypologyResults empty -> fail closed.
	})
	if withoutTypo.Status != domain.StatusAlert {
		t.Fatalf("without typology results: expected fail-closed ALRT, got %s", withoutTypo.Status)
	}
	if len(withoutTypo.TypologyResults) != 0 {
		t.Fatalf("without typology results: expected no synthetic TypologyResults, got %d", len(withoutTypo.TypologyResults))
	}
}

// TestComplianceModeMissedAlertDirection reproduces the missed-alert
// direction: a typology that WOULD trigger (ALRT in compliance) is not
// evaluated, and a low-scoring rule dilutes the aggregate below the detection
// threshold. The OLD code produced NALT (missed alert); the fix produces a
// fail-closed ALRT so the alert is no longer missed.
func TestComplianceModeMissedAlertDirection(t *testing.T) {
	proc := NewComplianceProcessor()
	proc.AlertThreshold = 0.7
	ctx := context.Background()

	// Rule A (in typology): score 1.0, weight 1.0
	// Rule B (not in typology): score 0.0, weight 4.0
	// Detection aggregate = (1.0*1.0 + 0.0*4.0) / (1.0 + 4.0) = 0.2 < 0.7 -> NALT
	// Typology score = 1.0 * 1.0 = 1.0 >= 0.5 -> triggered -> ALRT
	rs := []domain.RuleResult{
		{RuleID: "rule-a", Score: 1.0, SubRuleRef: domain.RuleOutcomeReview, Weight: 1.0},
		{RuleID: "rule-b", Score: 0.0, SubRuleRef: domain.RuleOutcomePass, Weight: 4.0},
	}
	typologyResults := []domain.TypologyResult{
		{TypologyID: "structuring-check", Score: 1.0, Threshold: 0.5, Triggered: true},
	}

	withTypo := proc.Process(ctx, &DecisionInput{
		TenantID:        "t",
		TxID:            "with",
		StartTime:       time.Now(),
		RuleResults:     rs,
		TypologyResults: typologyResults,
	})
	if withTypo.Status != domain.StatusAlert {
		t.Fatalf("with typology results: expected ALRT (typology triggered), got %s", withTypo.Status)
	}

	withoutTypo := proc.Process(ctx, &DecisionInput{
		TenantID:    "t",
		TxID:        "without",
		StartTime:   time.Now(),
		RuleResults: rs,
		// TypologyResults empty -> OLD: detection NALT (missed alert); FIX: fail-closed ALRT.
	})
	if withoutTypo.Status != domain.StatusAlert {
		t.Fatalf("without typology results: expected fail-closed ALRT (no missed alert), got %s", withoutTypo.Status)
	}
	// Detection aggregate is 0.2; prove we are NOT in detection mode (detection
	// mode would have been NALT). The fail-closed score carries the aggregate
	// for information only.
	if withoutTypo.Score > 0.3 {
		t.Logf("note: without-typology score=%.2f (aggregate, informational)", withoutTypo.Score)
	}
}
