package worker

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/opensource-finance/osprey/internal/bus"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// countingRepo counts SaveEvaluation calls so a test can prove a racy
// compliance-mode transaction does not persist a detection-mode decision.
type countingRepo struct {
	domain.Repository
	saved atomic.Int32
}

func (r *countingRepo) SaveEvaluation(_ context.Context, _ string, _ *domain.Evaluation) error {
	r.saved.Add(1)
	return nil
}

// TestWorkerComplianceTOCTOURaceRejectsOnEmptyTypologyMidRequest reproduces the
// worker-side TOCTOU race: a compliance-mode transaction passes the entry
// guard (typologies loaded), then blocks during rule evaluation (via a
// blocking velocity getter), then a concurrent admin operation empties the
// typology set. After the rule evaluation completes, the atomic generation
// check detects the change and the worker returns an error instead of
// producing a detection-mode decision.
func TestWorkerComplianceTOCTOURaceRejectsOnEmptyTypologyMidRequest(t *testing.T) {
	eventBus := bus.NewChannelBus(10)
	defer func() { _ = eventBus.Close() }()

	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once

	// A velocity getter that blocks EvaluateAll until released, creating the
	// TOCTOU window between the worker's entry guard and its typology
	// evaluation. The getter is invoked by EvaluateAll because the message
	// sets a non-zero VelocityWindow.
	getter := rules.VelocityGetter(func(_ context.Context, _, _ string, _ int) (int64, error) {
		once.Do(func() { close(reached) })
		<-release
		return 0, nil
	})

	engine, _ := rules.NewEngine(getter, 2)
	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "high-amount-rule", Name: "High Amount", Version: "1.0.0",
		Expression: "amount > 100.0 ? 0.8 : 0.0", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("failed to load rule: %v", err)
	}

	typologyEngine := rules.NewTypologyEngine()
	typologyEngine.LoadTypologies([]*domain.Typology{{
		ID:             "structuring-check",
		Name:           "Structuring",
		AlertThreshold: 0.9,
		Enabled:        true,
		Rules:          []domain.TypologyRuleWeight{{RuleID: "high-amount-rule", Weight: 1.0}},
	}})

	processor := tadp.NewComplianceProcessor()
	repo := &countingRepo{}

	w := NewWorker(eventBus, repo, engine, typologyEngine, processor, domain.ModeCompliance)

	payload, _ := json.Marshal(TransactionMessage{
		TxID:       "tx-worker-race",
		TenantID:   "tenant-001",
		Type:       "transfer",
		DebtorID:   "d",
		CreditorID: "c",
		Amount:     200.0, // -> rule score 0.8
		Currency:   "USD",
	})
	msg := &domain.Message{
		ID:       "msg-race",
		TenantID: "tenant-001",
		Topic:    domain.TopicTransactionIngested,
		Payload:  payload,
	}

	var (
		wg      sync.WaitGroup
		procErr error
	)
	wg.Go(func() {
		procErr = w.processTransaction(context.Background(), "tenant-001", msg, true)
	})

	<-reached // worker is past the entry guard, blocked in EvaluateAll

	// Concurrent admin operation empties the typology set.
	typologyEngine.LoadTypologies(nil)
	if typologyEngine.TypologyCount() != 0 {
		t.Fatalf("expected typology engine empty, got %d", typologyEngine.TypologyCount())
	}

	close(release)
	wg.Wait()

	if procErr == nil {
		t.Fatal("expected error when typology set changed mid-request; got nil")
	}
	if !strings.Contains(procErr.Error(), "typology configuration changed") {
		t.Fatalf("expected error about typology configuration change, got %v", procErr)
	}
	if got := repo.saved.Load(); got != 0 {
		t.Fatalf("racy compliance transaction must not persist an evaluation; got %d saved", got)
	}
}

// TestWorkerComplianceHappyPathStillEvaluates is a regression guard: with a
// stable typology set, a compliance-mode transaction is evaluated through the
// typology path and persists exactly one evaluation (NALT when the typology is
// below threshold).
func TestWorkerComplianceHappyPathStillEvaluates(t *testing.T) {
	eventBus := bus.NewChannelBus(10)
	defer func() { _ = eventBus.Close() }()

	// Non-blocking getter so EvaluateAll completes immediately.
	engine, _ := rules.NewEngine(rules.VelocityGetter(func(_ context.Context, _, _ string, _ int) (int64, error) {
		return 0, nil
	}), 2)
	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "high-amount-rule", Name: "High Amount", Version: "1.0.0",
		Expression: "amount > 100.0 ? 0.8 : 0.0", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("failed to load rule: %v", err)
	}

	typologyEngine := rules.NewTypologyEngine()
	typologyEngine.LoadTypologies([]*domain.Typology{{
		ID:             "structuring-check",
		Name:           "Structuring",
		AlertThreshold: 0.9,
		Enabled:        true,
		Rules:          []domain.TypologyRuleWeight{{RuleID: "high-amount-rule", Weight: 1.0}},
	}})

	processor := tadp.NewComplianceProcessor()
	repo := &countingRepo{}

	w := NewWorker(eventBus, repo, engine, typologyEngine, processor, domain.ModeCompliance)

	payload, _ := json.Marshal(TransactionMessage{
		TxID:     "tx-happy",
		TenantID: "tenant-001",
		Type:     "transfer",
		DebtorID: "d", CreditorID: "c",
		Amount: 200.0, Currency: "USD",
	})
	msg := &domain.Message{ID: "msg-happy", TenantID: "tenant-001", Topic: domain.TopicTransactionIngested, Payload: payload}

	if err := w.processTransaction(context.Background(), "tenant-001", msg, true); err != nil {
		t.Fatalf("happy path: expected no error, got %v", err)
	}
	if got := repo.saved.Load(); got != 1 {
		t.Fatalf("happy path: expected exactly 1 evaluation persisted, got %d", got)
	}
}

// TestWorkerComplianceNonEmptyReplacementRejects proves the worker generation
// check also catches a non-empty typology replacement (not just cleared),
// because any mid-request change invalidates the compliance semantics.
func TestWorkerComplianceNonEmptyReplacementRejects(t *testing.T) {
	eventBus := bus.NewChannelBus(10)
	defer func() { _ = eventBus.Close() }()

	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	getter := rules.VelocityGetter(func(_ context.Context, _, _ string, _ int) (int64, error) {
		once.Do(func() { close(reached) })
		<-release
		return 0, nil
	})
	engine, _ := rules.NewEngine(getter, 2)
	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "high-amount-rule", Name: "High Amount", Version: "1.0.0",
		Expression: "amount > 100.0 ? 0.8 : 0.0", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("failed to load rule: %v", err)
	}

	typologyEngine := rules.NewTypologyEngine()
	typologyEngine.LoadTypologies([]*domain.Typology{{
		ID:             "structuring-check",
		Name:           "Structuring",
		AlertThreshold: 0.9,
		Enabled:        true,
		Rules:          []domain.TypologyRuleWeight{{RuleID: "high-amount-rule", Weight: 1.0}},
	}})

	processor := tadp.NewComplianceProcessor()
	repo := &countingRepo{}
	w := NewWorker(eventBus, repo, engine, typologyEngine, processor, domain.ModeCompliance)

	payload, _ := json.Marshal(TransactionMessage{TxID: "tx-repl", TenantID: "tenant-001", Type: "transfer", DebtorID: "d", CreditorID: "c", Amount: 200.0, Currency: "USD"})
	msg := &domain.Message{ID: "msg-repl", TenantID: "tenant-001", Topic: domain.TopicTransactionIngested, Payload: payload}

	var (
		wg      sync.WaitGroup
		procErr error
	)
	wg.Go(func() {
		procErr = w.processTransaction(context.Background(), "tenant-001", msg, true)
	})
	<-reached

	// Replace with a different, still non-empty set.
	typologyEngine.LoadTypologies([]*domain.Typology{{
		ID:             "totally-different",
		Name:           "Different",
		AlertThreshold: 0.5,
		Enabled:        true,
		Rules:          []domain.TypologyRuleWeight{{RuleID: "other-rule", Weight: 1.0}},
	}})

	close(release)
	wg.Wait()

	if procErr == nil {
		t.Fatal("expected error when typology set was replaced mid-request; got nil")
	}
	if got := repo.saved.Load(); got != 0 {
		t.Fatalf("replaced-set transaction must not persist an evaluation; got %d saved", got)
	}
}
