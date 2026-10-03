package worker

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/bus"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// capturedEval records a single SaveEvaluation call.
type capturedEval struct {
	TenantID string
	Eval     *domain.Evaluation
}

// capturingRepository records SaveEvaluation calls without persisting.
// Embedding the nil domain.Repository satisfies the interface; only
// SaveEvaluation is exercised here, mirroring the failingEvaluationRepository
// pattern in worker_test.go.
type capturingRepository struct {
	domain.Repository
	mu    sync.Mutex
	saved []capturedEval
}

func (r *capturingRepository) SaveEvaluation(_ context.Context, tenantID string, eval *domain.Evaluation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved = append(r.saved, capturedEval{TenantID: tenantID, Eval: eval})
	return nil
}

func (r *capturingRepository) savedEvals() []capturedEval {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]capturedEval, len(r.saved))
	copy(out, r.saved)
	return out
}

// capturedPublish records a single Publish call.
type capturedPublish struct {
	TenantID string
	Topic    string
	Payload  []byte
}

// capturingBus records Publish calls without dispatching.
type capturingBus struct {
	domain.EventBus
	mu        sync.Mutex
	publishes []capturedPublish
}

func (b *capturingBus) Publish(_ context.Context, tenantID, topic string, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.publishes = append(b.publishes, capturedPublish{TenantID: tenantID, Topic: topic, Payload: payload})
	return nil
}

func (b *capturingBus) published() []capturedPublish {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]capturedPublish, len(b.publishes))
	copy(out, b.publishes)
	return out
}

func countPublishes(pub []capturedPublish, tenantID, topic string) int {
	n := 0
	for _, p := range pub {
		if p.TenantID == tenantID && p.Topic == topic {
			n++
		}
	}
	return n
}

func publishesForTenant(pub []capturedPublish, tenantID string) int {
	n := 0
	for _, p := range pub {
		if p.TenantID == tenantID {
			n++
		}
	}
	return n
}

// noAlertRule yields a clean NALT (score 0, no alert) so only a decision is
// published, keeping decision-routing assertions free of alert noise.
func noAlertRule() []*domain.RuleConfig {
	return []*domain.RuleConfig{
		{
			ID:         "no-fire",
			Name:       "No Fire",
			Expression: "amount > 1000000.0",
			Weight:     1.0,
			Enabled:    true,
		},
	}
}

// alertRule yields an ALRT when debtor == creditor (same-party), matching the
// proven pattern from the AlertPublished test in worker_test.go.
func alertRule() []*domain.RuleConfig {
	return []*domain.RuleConfig{
		{
			ID:         "same-party-check",
			Name:       "Same Party Check",
			Expression: "debtor_id == creditor_id",
			Weight:     1.0,
			Enabled:    true,
		},
	}
}

func alertProcessor() *tadp.Processor {
	return &tadp.Processor{
		AlertThreshold:     0.1,
		UseWeightedScoring: true,
	}
}

func marshalTxMsg(t *testing.T, txMsg TransactionMessage) []byte {
	t.Helper()
	b, err := json.Marshal(txMsg)
	if err != nil {
		t.Fatalf("marshal transaction message: %v", err)
	}
	return b
}

func newCapturingWorker(t *testing.T, processor *tadp.Processor, ruleConfigs []*domain.RuleConfig) (*Worker, *capturingRepository, *capturingBus) {
	t.Helper()
	engine, _ := rules.NewEngine(nil, 5)
	if err := engine.LoadRules(ruleConfigs); err != nil {
		t.Fatalf("load rules: %v", err)
	}
	repo := &capturingRepository{}
	busCap := &capturingBus{}
	w := NewWorker(busCap, repo, engine, rules.NewTypologyEngine(), processor, domain.ModeDetection)
	return w, repo, busCap
}

// ---------------------------------------------------------------------------
// Direct unit tests against processTransaction's trustPayloadTenant flag.
// ---------------------------------------------------------------------------

// Reproduce the reported bug: a per-tenant (authoritative) call must NOT let
// the payload tenantId override the subscription tenant for SaveEvaluation or
// for decision/alert publishing.
func TestProcessTransaction_PerTenantAuthoritative_IgnoresPayloadTenantOverride(t *testing.T) {
	w, repo, busCap := newCapturingWorker(t, tadp.NewProcessor(), noAlertRule())

	payload := marshalTxMsg(t, TransactionMessage{
		TxID:       "tx-cross",
		TenantID:   "tenantB", // untrusted payload tenant, differs from subscription
		TraceID:    "trace-1",
		Type:       "transfer",
		DebtorID:   "debtor-001",
		CreditorID: "creditor-001",
		Amount:     500.0,
		Currency:   "USD",
	})

	err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID:       "msg-1",
		TenantID: "tenantA",
		Topic:    domain.TopicTransactionIngested,
		Payload:  payload,
	}, false)
	if err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 {
		t.Fatalf("expected 1 SaveEvaluation, got %d", len(saved))
	}
	if saved[0].TenantID != "tenantA" {
		t.Errorf("SaveEvaluation tenant = %q, want %q (subscription tenant must win)", saved[0].TenantID, "tenantA")
	}
	if saved[0].Eval.TenantID != "tenantA" {
		t.Errorf("evaluation.TenantID = %q, want %q", saved[0].Eval.TenantID, "tenantA")
	}

	pub := busCap.published()
	if got := countPublishes(pub, "tenantA", domain.TopicDecision); got != 1 {
		t.Errorf("decision publishes on tenantA = %d, want 1", got)
	}
	if got := publishesForTenant(pub, "tenantB"); got != 0 {
		t.Errorf("publishes on tenantB = %d, want 0 (payload tenant must not receive any event)", got)
	}
	if got := countPublishes(pub, "tenantB", domain.TopicDecision); got != 0 {
		t.Errorf("decision publishes on tenantB = %d, want 0", got)
	}
}

// No regression when payload tenant agrees with the subscription tenant.
func TestProcessTransaction_PerTenantAuthoritative_MatchingPayloadTenant(t *testing.T) {
	w, repo, busCap := newCapturingWorker(t, tadp.NewProcessor(), noAlertRule())

	payload := marshalTxMsg(t, TransactionMessage{
		TxID:       "tx-match",
		TenantID:   "tenantA",
		Type:       "transfer",
		DebtorID:   "d1",
		CreditorID: "c1",
		Amount:     500.0,
		Currency:   "USD",
	})

	if err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-2", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false); err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 || saved[0].TenantID != "tenantA" {
		t.Fatalf("SaveEvaluation tenant = %+v, want tenantA", saved)
	}
	if got := countPublishes(busCap.published(), "tenantA", domain.TopicDecision); got != 1 {
		t.Errorf("decision publishes on tenantA = %d, want 1", got)
	}
}

// Empty payload tenantId: subscription tenant is used, no override, no warning.
func TestProcessTransaction_PerTenantAuthoritative_EmptyPayloadTenant(t *testing.T) {
	w, repo, busCap := newCapturingWorker(t, tadp.NewProcessor(), noAlertRule())

	payload := marshalTxMsg(t, TransactionMessage{
		TxID:       "tx-empty",
		TenantID:   "", // payload omits tenantId
		Type:       "transfer",
		DebtorID:   "d1",
		CreditorID: "c1",
		Amount:     500.0,
		Currency:   "USD",
	})

	if err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-3", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false); err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 || saved[0].TenantID != "tenantA" {
		t.Fatalf("SaveEvaluation tenant = %+v, want tenantA", saved)
	}
	if got := countPublishes(busCap.published(), "tenantA", domain.TopicDecision); got != 1 {
		t.Errorf("decision publishes on tenantA = %d, want 1", got)
	}
}

// Alert routing must also follow the subscription tenant, not the payload tenant.
func TestProcessTransaction_PerTenantAuthoritative_AlertRoutedToSubscriptionTenant(t *testing.T) {
	w, repo, busCap := newCapturingWorker(t, alertProcessor(), alertRule())

	payload := marshalTxMsg(t, TransactionMessage{
		TxID:       "tx-alert",
		TenantID:   "tenantB", // differs from subscription
		Type:       "transfer",
		DebtorID:   "same-user",
		CreditorID: "same-user", // triggers same-party rule -> ALRT
		Amount:     100.0,
		Currency:   "USD",
	})

	if err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-4", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false); err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 || saved[0].TenantID != "tenantA" {
		t.Fatalf("SaveEvaluation tenant = %+v, want tenantA", saved)
	}
	if saved[0].Eval.Status != domain.StatusAlert {
		t.Fatalf("expected ALRT evaluation, got %q", saved[0].Eval.Status)
	}

	pub := busCap.published()
	if got := countPublishes(pub, "tenantA", domain.TopicDecision); got != 1 {
		t.Errorf("decision publishes on tenantA = %d, want 1", got)
	}
	if got := countPublishes(pub, "tenantB", domain.TopicAlert); got != 0 {
		t.Errorf("alert publishes on tenantB = %d, want 0 (alert must route to subscription tenant)", got)
	}
	if got := countPublishes(pub, "tenantA", domain.TopicAlert); got != 1 {
		t.Errorf("alert publishes on tenantA = %d, want 1", got)
	}
	if got := publishesForTenant(pub, "tenantB"); got != 0 {
		t.Errorf("publishes on tenantB = %d, want 0", got)
	}
}

// Global (testing/dev) path must still recover the real tenant from the payload.
func TestProcessTransaction_GlobalTrustsPayloadTenant(t *testing.T) {
	w, repo, busCap := newCapturingWorker(t, tadp.NewProcessor(), noAlertRule())

	payload := marshalTxMsg(t, TransactionMessage{
		TxID:       "tx-global",
		TenantID:   "tenantX", // global worker's only source of the real tenant
		Type:       "transfer",
		DebtorID:   "d1",
		CreditorID: "c1",
		Amount:     500.0,
		Currency:   "USD",
	})

	if err := w.processTransaction(context.Background(), "_global", &domain.Message{
		ID: "msg-5", TenantID: "_global", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, true); err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 || saved[0].TenantID != "tenantX" {
		t.Fatalf("SaveEvaluation tenant = %+v, want tenantX (global path must recover tenant from payload)", saved)
	}
	if got := countPublishes(busCap.published(), "tenantX", domain.TopicDecision); got != 1 {
		t.Errorf("decision publishes on tenantX = %d, want 1", got)
	}
	if got := countPublishes(busCap.published(), "_global", domain.TopicDecision); got != 0 {
		t.Errorf("decision publishes on _global = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// End-to-end test through a real ChannelBus and a started Worker.
// ---------------------------------------------------------------------------

func subscribeChan(t *testing.T, b domain.EventBus, tenantID, topic string) <-chan *domain.Message {
	t.Helper()
	ch := make(chan *domain.Message, 16)
	_, err := b.Subscribe(context.Background(), tenantID, topic, func(_ context.Context, msg *domain.Message) error {
		select {
		case ch <- msg:
		default:
		}
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe %s:%s: %v", tenantID, topic, err)
	}
	return ch
}

func recvMsg(t *testing.T, ch <-chan *domain.Message, timeout time.Duration, label string) *domain.Message {
	t.Helper()
	select {
	case m := <-ch:
		return m
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for message on %s", label)
		return nil
	}
}

func assertNoMsg(t *testing.T, ch <-chan *domain.Message, label string) {
	t.Helper()
	select {
	case m := <-ch:
		t.Errorf("unexpected message on %s: txId=%s tenant=%s", label, m.ID, m.TenantID)
	case <-time.After(250 * time.Millisecond):
		// expected: no message
	}
}

// End-to-end repro of the reported scenario: a message published to tenantA's
// ingested subject whose payload claims tenantB is attributed to tenantA.
func TestPerTenantWorker_E2E_IgnoresPayloadTenantOverride(t *testing.T) {
	eventBus := bus.NewChannelBus(100)
	defer func() { _ = eventBus.Close() }()

	engine, _ := rules.NewEngine(nil, 5)
	_ = engine.LoadRules(noAlertRule())

	repo := &capturingRepository{}
	w := NewWorker(eventBus, repo, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), domain.ModeDetection)
	if err := w.Start(Config{TenantIDs: []string{"tenantA", "tenantB"}}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = w.Stop() }()

	decA := subscribeChan(t, eventBus, "tenantA", domain.TopicDecision)
	decB := subscribeChan(t, eventBus, "tenantB", domain.TopicDecision)

	time.Sleep(50 * time.Millisecond) // let subscriptions register

	payload := marshalTxMsg(t, TransactionMessage{
		TxID:       "tx-e2e-cross",
		TenantID:   "tenantB", // payload claims tenantB
		Type:       "transfer",
		DebtorID:   "d1",
		CreditorID: "c1",
		Amount:     500.0,
		Currency:   "USD",
	})
	if err := eventBus.Publish(context.Background(), "tenantA", domain.TopicTransactionIngested, payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	recvMsg(t, decA, 2*time.Second, "tenantA:decision")
	assertNoMsg(t, decB, "tenantB:decision")

	saved := repo.savedEvals()
	if len(saved) != 1 {
		t.Fatalf("expected 1 SaveEvaluation, got %d", len(saved))
	}
	if saved[0].TenantID != "tenantA" {
		t.Errorf("SaveEvaluation tenant = %q, want tenantA", saved[0].TenantID)
	}
}
