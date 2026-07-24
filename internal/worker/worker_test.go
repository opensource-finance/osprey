package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/bus"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

type failingEvaluationRepository struct {
	domain.Repository
	err error
}

func (r *failingEvaluationRepository) SaveEvaluation(_ context.Context, _ string, _ *domain.Evaluation) error {
	return r.err
}

type failingPublishBus struct {
	domain.EventBus
	err error
}

func (b *failingPublishBus) Publish(_ context.Context, _ string, _ string, _ []byte) error {
	return b.err
}

func TestWorker(t *testing.T) {
	// Create channel bus
	eventBus := bus.NewChannelBus(100)
	defer func() { _ = eventBus.Close() }()

	// Create rule engine with test rules (no hardcoded builtin rules)
	engine, _ := rules.NewEngine(nil, 5)

	// Load test rules for worker tests
	testRules := []*domain.RuleConfig{
		{
			ID:         "test-rule-001",
			Name:       "Test Rule",
			Expression: "amount > 0.0",
			Weight:     1.0,
			Enabled:    true,
		},
		{
			ID:         "same-party-check",
			Name:       "Same Party Check",
			Expression: "debtor_id == creditor_id",
			Weight:     1.0,
			Enabled:    true,
		},
	}
	_ = engine.LoadRules(testRules)

	// Create typology engine
	typologyEngine := rules.NewTypologyEngine()

	// Create processor
	processor := tadp.NewProcessor()

	// Create worker
	worker := NewWorker(eventBus, nil, engine, typologyEngine, processor, domain.ModeDetection)

	t.Run("StartAndStop", func(t *testing.T) {
		cfg := Config{
			TenantIDs:   []string{"tenant-001"},
			WorkerCount: 1,
		}

		err := worker.Start(cfg)
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		stats := worker.GetStats()
		if stats.SubscriptionCount != 1 {
			t.Errorf("expected 1 subscription, got %d", stats.SubscriptionCount)
		}

		err = worker.Stop()
		if err != nil {
			t.Errorf("Stop failed: %v", err)
		}

		stats = worker.GetStats()
		if stats.SubscriptionCount != 0 {
			t.Errorf("expected 0 subscriptions after stop, got %d", stats.SubscriptionCount)
		}
	})

	t.Run("ProcessTransaction", func(t *testing.T) {
		// Create fresh worker for this test
		w := NewWorker(eventBus, nil, engine, typologyEngine, processor, domain.ModeDetection)

		cfg := Config{
			TenantIDs: []string{"tenant-test"},
		}
		_ = w.Start(cfg)
		defer func() { _ = w.Stop() }()

		// Track decision results
		var decisionReceived atomic.Bool
		var decisionPayload []byte

		_, _ = eventBus.Subscribe(context.Background(), "tenant-test", domain.TopicDecision, func(ctx context.Context, msg *domain.Message) error {
			decisionPayload = msg.Payload
			decisionReceived.Store(true)
			return nil
		})

		// Allow subscriptions to be active
		time.Sleep(50 * time.Millisecond)

		// Publish a transaction
		txMsg := TransactionMessage{
			TxID:       "tx-001",
			TenantID:   "tenant-test",
			TraceID:    "trace-001",
			Type:       "transfer",
			DebtorID:   "debtor-001",
			CreditorID: "creditor-001",
			Amount:     500.0,
			Currency:   "USD",
		}

		payload, _ := json.Marshal(txMsg)
		err := eventBus.Publish(context.Background(), "tenant-test", domain.TopicTransactionIngested, payload)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		if !decisionReceived.Load() {
			t.Error("expected decision to be published")
		}

		if decisionPayload != nil {
			var eval domain.Evaluation
			if err := json.Unmarshal(decisionPayload, &eval); err != nil {
				t.Fatalf("failed to parse decision: %v", err)
			}

			if eval.TxID != "tx-001" {
				t.Errorf("expected txID 'tx-001', got '%s'", eval.TxID)
			}
			if eval.TenantID != "tenant-test" {
				t.Errorf("expected tenantID 'tenant-test', got '%s'", eval.TenantID)
			}
			if eval.Metadata.TraceID != "trace-001" {
				t.Errorf("expected traceID 'trace-001', got '%s'", eval.Metadata.TraceID)
			}
		}
	})

	t.Run("AlertPublished", func(t *testing.T) {
		// Create worker with a low threshold processor
		lowThresholdProcessor := &tadp.Processor{
			AlertThreshold:     0.1, // Very low threshold
			UseWeightedScoring: true,
		}

		w := NewWorker(eventBus, nil, engine, typologyEngine, lowThresholdProcessor, domain.ModeDetection)

		cfg := Config{
			TenantIDs: []string{"tenant-alert"},
		}
		_ = w.Start(cfg)
		defer func() { _ = w.Stop() }()

		// Track alerts
		var alertReceived atomic.Bool

		_, _ = eventBus.Subscribe(context.Background(), "tenant-alert", domain.TopicAlert, func(ctx context.Context, msg *domain.Message) error {
			alertReceived.Store(true)
			return nil
		})

		time.Sleep(50 * time.Millisecond)

		// Publish a high-risk transaction (same debtor/creditor triggers rule)
		txMsg := TransactionMessage{
			TxID:       "tx-alert",
			TenantID:   "tenant-alert",
			Type:       "transfer",
			DebtorID:   "same-user", // Same as creditor
			CreditorID: "same-user",
			Amount:     100.0,
			Currency:   "USD",
		}

		payload, _ := json.Marshal(txMsg)
		_ = eventBus.Publish(context.Background(), "tenant-alert", domain.TopicTransactionIngested, payload)

		time.Sleep(100 * time.Millisecond)

		if !alertReceived.Load() {
			t.Error("expected alert to be published for high-risk transaction")
		}
	})

	t.Run("MultiTenant", func(t *testing.T) {
		w := NewWorker(eventBus, nil, engine, typologyEngine, processor, domain.ModeDetection)

		cfg := Config{
			TenantIDs: []string{"tenant-a", "tenant-b"},
		}
		_ = w.Start(cfg)
		defer func() { _ = w.Stop() }()

		stats := w.GetStats()
		if stats.SubscriptionCount != 2 {
			t.Errorf("expected 2 subscriptions for 2 tenants, got %d", stats.SubscriptionCount)
		}
	})
}

func TestTransactionMessageParsing(t *testing.T) {
	msg := TransactionMessage{
		TxID:           "tx-123",
		TenantID:       "tenant-001",
		TraceID:        "trace-456",
		Type:           "transfer",
		DebtorID:       "debtor-001",
		CreditorID:     "creditor-001",
		Amount:         1234.56,
		Currency:       "USD",
		VelocityWindow: 7200,
		AdditionalData: map[string]any{"key": "value"},
	}

	// Marshal and unmarshal
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed TransactionMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.TxID != msg.TxID {
		t.Errorf("expected TxID '%s', got '%s'", msg.TxID, parsed.TxID)
	}
	if parsed.Amount != msg.Amount {
		t.Errorf("expected Amount %.2f, got %.2f", msg.Amount, parsed.Amount)
	}
	if parsed.VelocityWindow != msg.VelocityWindow {
		t.Errorf("expected VelocityWindow %d, got %d", msg.VelocityWindow, parsed.VelocityWindow)
	}
}

func TestProcessTransaction_ComplianceModeRequiresTypologies(t *testing.T) {
	eventBus := bus.NewChannelBus(10)
	defer func() { _ = eventBus.Close() }()

	engine, _ := rules.NewEngine(nil, 2)
	_ = engine.LoadRules([]*domain.RuleConfig{
		{
			ID:         "test-rule-001",
			Name:       "Test Rule",
			Expression: "amount > 0.0",
			Weight:     1.0,
			Enabled:    true,
		},
	})

	typologyEngine := rules.NewTypologyEngine() // intentionally empty
	processor := tadp.NewComplianceProcessor()

	w := NewWorker(eventBus, nil, engine, typologyEngine, processor, domain.ModeCompliance)

	payload, _ := json.Marshal(TransactionMessage{
		TxID:       "tx-compliance-no-typologies",
		TenantID:   "tenant-001",
		Type:       "transfer",
		DebtorID:   "debtor-001",
		CreditorID: "creditor-001",
		Amount:     100,
		Currency:   "USD",
	})

	err := w.processTransaction(context.Background(), "tenant-001", &domain.Message{
		ID:       "msg-001",
		TenantID: "tenant-001",
		Topic:    domain.TopicTransactionIngested,
		Payload:  payload,
	})
	if err == nil {
		t.Fatal("expected error when compliance mode has no typologies")
	}
}

func TestProcessTransactionReturnsPipelineFailures(t *testing.T) {
	engine, _ := rules.NewEngine(nil, 2)
	if err := engine.LoadRule(&domain.RuleConfig{
		ID:         "test-rule-001",
		Name:       "Test Rule",
		Expression: "amount > 0.0",
		Weight:     1.0,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("failed to load test rule: %v", err)
	}

	payload, err := json.Marshal(TransactionMessage{
		TxID:       "tx-failure",
		TenantID:   "tenant-001",
		Type:       "transfer",
		DebtorID:   "debtor-001",
		CreditorID: "creditor-001",
		Amount:     100,
		Currency:   "USD",
	})
	if err != nil {
		t.Fatalf("failed to marshal transaction message: %v", err)
	}
	msg := &domain.Message{
		ID:       "msg-failure",
		TenantID: "tenant-001",
		Topic:    domain.TopicTransactionIngested,
		Payload:  payload,
	}

	t.Run("SaveEvaluationFailure", func(t *testing.T) {
		eventBus := bus.NewChannelBus(10)
		defer func() { _ = eventBus.Close() }()

		w := NewWorker(
			eventBus,
			&failingEvaluationRepository{err: errors.New("repository unavailable")},
			engine,
			rules.NewTypologyEngine(),
			tadp.NewProcessor(),
			domain.ModeDetection,
		)

		err := w.processTransaction(context.Background(), "tenant-001", msg)
		if err == nil || !strings.Contains(err.Error(), "failed to save evaluation") {
			t.Fatalf("expected save evaluation error, got %v", err)
		}
	})

	t.Run("PublishDecisionFailure", func(t *testing.T) {
		w := NewWorker(
			&failingPublishBus{err: errors.New("bus unavailable")},
			nil,
			engine,
			rules.NewTypologyEngine(),
			tadp.NewProcessor(),
			domain.ModeDetection,
		)

		err := w.processTransaction(context.Background(), "tenant-001", msg)
		if err == nil || !strings.Contains(err.Error(), "failed to publish decision") {
			t.Fatalf("expected publish decision error, got %v", err)
		}
	})
}
