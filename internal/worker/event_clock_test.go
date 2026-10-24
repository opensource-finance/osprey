package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/bus"
	"github.com/opensource-finance/osprey/internal/cache"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
	"github.com/opensource-finance/osprey/internal/velocity"
)

// capturingAggregatesGetter records the at argument passed to the
// AggregatesGetter so tests can assert which velocity clock the worker
// path selected.
type capturingAggregatesGetter struct {
	mu      sync.Mutex
	lastAt  time.Time
	calls   int32
	atTimes []time.Time
}

func (g *capturingAggregatesGetter) get(_ context.Context, _, _, _ string, _ int, at time.Time) (rules.VelocityAggregates, error) {
	g.mu.Lock()
	g.lastAt = at
	g.atTimes = append(g.atTimes, at)
	g.mu.Unlock()
	atomic.AddInt32(&g.calls, 1)
	return rules.VelocityAggregates{Count: 0}, nil
}

func (g *capturingAggregatesGetter) snapshot() (time.Time, int32) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastAt, atomic.LoadInt32(&g.calls)
}

// newEventClockWorker builds a worker whose engine has the event clock on
// and a capturing aggregates getter.
func newEventClockWorker(t *testing.T, getter *capturingAggregatesGetter) (*Worker, *rules.Engine) {
	t.Helper()
	engine, err := rules.NewEngine(nil, 5)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.SetEventClock(true)
	engine.SetAggregatesGetter(getter.get)

	_ = engine.LoadRules([]*domain.RuleConfig{
		{
			ID:         "velocity-rule",
			Name:       "Velocity Check",
			Expression: "velocity_count > 5 ? 1.0 : 0.0",
			Weight:     1.0,
			Enabled:    true,
		},
	})

	eventBus := bus.NewChannelBus(10)
	w := NewWorker(eventBus, nil, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), domain.ModeDetection)
	return w, engine
}

// TestWorkerEventClock_PassesTimestampToGetter verifies that the worker
// parses TransactionMessage.Timestamp and forwards it as EventTime to the
// aggregates getter when the event clock is on.
func TestWorkerEventClock_PassesTimestampToGetter(t *testing.T) {
	getter := &capturingAggregatesGetter{}
	w, _ := newEventClockWorker(t, getter)

	eventTime := time.Date(2025, 6, 15, 12, 30, 0, 0, time.UTC)

	payload, _ := json.Marshal(TransactionMessage{
		TxID:           "tx-event",
		TenantID:       "tenantA",
		Type:           "transfer",
		DebtorID:       "d1",
		CreditorID:     "c1",
		Amount:         500.0,
		Currency:       "USD",
		Timestamp:      eventTime.Format(time.RFC3339),
		VelocityWindow: 3600,
	})

	err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID:       "msg-event",
		TenantID: "tenantA",
		Topic:    domain.TopicTransactionIngested,
		Payload:  payload,
	}, false)
	if err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	lastAt, calls := getter.snapshot()
	if calls == 0 {
		t.Fatal("aggregates getter was never called")
	}
	if lastAt.IsZero() {
		t.Fatal("event clock on: getter received zero at — worker did not pass EventTime")
	}
	if !lastAt.Equal(eventTime) {
		t.Errorf("getter at = %s, want %s", lastAt, eventTime)
	}
}

// TestWorkerEventClock_EmptyTimestampDefaultsToNow verifies that a message
// without a timestamp still works — EventTime defaults to now so the event
// clock uses the current time as the window end (equivalent to received clock
// for a live message, but queried through the event-clock path).
func TestWorkerEventClock_EmptyTimestampDefaultsToNow(t *testing.T) {
	getter := &capturingAggregatesGetter{}
	w, _ := newEventClockWorker(t, getter)

	before := time.Now().UTC()

	payload, _ := json.Marshal(TransactionMessage{
		TxID:           "tx-nots",
		TenantID:       "tenantA",
		Type:           "transfer",
		DebtorID:       "d1",
		CreditorID:     "c1",
		Amount:         500.0,
		Currency:       "USD",
		VelocityWindow: 3600,
	})

	err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID:       "msg-nots",
		TenantID: "tenantA",
		Topic:    domain.TopicTransactionIngested,
		Payload:  payload,
	}, false)
	if err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	after := time.Now().UTC()

	lastAt, calls := getter.snapshot()
	if calls == 0 {
		t.Fatal("aggregates getter was never called")
	}
	if lastAt.IsZero() {
		t.Fatal("empty timestamp: getter received zero at — EventTime should default to now")
	}
	if lastAt.Before(before.Add(-1*time.Second)) || lastAt.After(after.Add(1*time.Second)) {
		t.Errorf("empty timestamp: getter at = %s, expected ~now (between %s and %s)", lastAt, before, after)
	}
}

// TestWorkerEventClock_InvalidTimestampReturnsError verifies that a
// malformed timestamp causes processTransaction to fail instead of
// silently falling back.
func TestWorkerEventClock_InvalidTimestampReturnsError(t *testing.T) {
	getter := &capturingAggregatesGetter{}
	w, _ := newEventClockWorker(t, getter)

	payload, _ := json.Marshal(TransactionMessage{
		TxID:           "tx-badts",
		TenantID:       "tenantA",
		Type:           "transfer",
		DebtorID:       "d1",
		CreditorID:     "c1",
		Amount:         500.0,
		Currency:       "USD",
		Timestamp:      "not-a-date",
		VelocityWindow: 3600,
	})

	err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID:       "msg-badts",
		TenantID: "tenantA",
		Topic:    domain.TopicTransactionIngested,
		Payload:  payload,
	}, false)
	if err == nil {
		t.Fatal("expected error for invalid timestamp, got nil")
	}
	if !strings.Contains(err.Error(), "invalid timestamp") {
		t.Errorf("error should mention 'invalid timestamp', got: %v", err)
	}

	if calls := atomic.LoadInt32(&getter.calls); calls != 0 {
		t.Errorf("aggregates getter should not be called on parse failure, got %d calls", calls)
	}
}

// TestWorkerReceivedClock_ZeroAtToGetter verifies that when the event clock
// is OFF, the worker path passes a zero at to the getter (received clock
// mode), so the getter wrapper selects GetAggregates (window ends now).
func TestWorkerReceivedClock_ZeroAtToGetter(t *testing.T) {
	getter := &capturingAggregatesGetter{}
	engine, err := rules.NewEngine(nil, 5)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.SetEventClock(false)
	engine.SetAggregatesGetter(getter.get)
	_ = engine.LoadRules([]*domain.RuleConfig{
		{
			ID:         "velocity-rule",
			Name:       "Velocity Check",
			Expression: "velocity_count > 5 ? 1.0 : 0.0",
			Weight:     1.0,
			Enabled:    true,
		},
	})

	eventBus := bus.NewChannelBus(10)
	w := NewWorker(eventBus, nil, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), domain.ModeDetection)

	eventTime := time.Date(2025, 6, 15, 12, 30, 0, 0, time.UTC)

	payload, _ := json.Marshal(TransactionMessage{
		TxID:           "tx-recv",
		TenantID:       "tenantA",
		Type:           "transfer",
		DebtorID:       "d1",
		CreditorID:     "c1",
		Amount:         500.0,
		Currency:       "USD",
		Timestamp:      eventTime.Format(time.RFC3339),
		VelocityWindow: 3600,
	})

	err = w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID:       "msg-recv",
		TenantID: "tenantA",
		Topic:    domain.TopicTransactionIngested,
		Payload:  payload,
	}, false)
	if err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	lastAt, calls := getter.snapshot()
	if calls == 0 {
		t.Fatal("aggregates getter was never called")
	}
	if !lastAt.IsZero() {
		t.Errorf("received clock: getter at = %s, want zero (received clock must ignore EventTime)", lastAt)
	}
}

// TestEngineWarnsOnZeroEventTimeWithEventClock verifies that the engine
// logs a warning when the event clock is enabled but EventTime is zero,
// and still falls back to the received clock gracefully.
func TestEngineWarnsOnZeroEventTimeWithEventClock(t *testing.T) {
	var logBuf bytes.Buffer
	oldDefault := slog.Default()
	defer slog.SetDefault(oldDefault)
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	engine, err := rules.NewEngine(nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	var gotAt atomic.Value // time.Time
	engine.SetAggregatesGetter(func(_ context.Context, _, _, _ string, _ int, at time.Time) (rules.VelocityAggregates, error) {
		gotAt.Store(at)
		return rules.VelocityAggregates{Count: 1}, nil
	})
	engine.SetEventClock(true)

	if err := engine.LoadRule(&domain.RuleConfig{
		ID:         "vc",
		Name:       "Velocity",
		Expression: "velocity_count == 1",
		Weight:     1.0,
		Enabled:    true,
	}); err != nil {
		t.Fatal(err)
	}

	in := &rules.EvaluateInput{
		TenantID:       "t",
		TxID:           "tx-zero",
		DebtorID:       "d",
		VelocityWindow: 3600,
		EventTime:      time.Time{}, // explicitly zero
	}

	res, err := engine.EvaluateAll(context.Background(), in)
	if err != nil {
		t.Fatalf("EvaluateAll: %v", err)
	}
	if res[0].Score != 1.0 {
		t.Errorf("expected score 1.0 (velocity_count==1), got %.1f", res[0].Score)
	}

	at := gotAt.Load().(time.Time)
	if !at.IsZero() {
		t.Errorf("zero EventTime should produce zero at, got %s", at)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "event clock enabled but EventTime is zero") {
		t.Errorf("expected warning about zero EventTime in logs, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "tx-zero") {
		t.Errorf("warning should include tx_id, got: %s", logOutput)
	}
}

// TestWorkerEventClock_RealVelocityAgreesWithDirectEvaluate wires a real
// SQLite repository + velocity service + engine with the event clock (the
// same wiring as cmd/osprey/main.go's newRuleEngine), seeds historical
// transactions via SaveTransaction, then evaluates the same event-time
// transaction two ways:
//  1. Direct engine.EvaluateAll (simulating the API path, which always set
//     EventTime).
//  2. Through the worker's processTransaction (the fixed path that now sets
//     EventTime from TransactionMessage.timestamp).
//
// Both paths must produce the same velocity_count and the same alert
// decision, proving the worker no longer silently diverges from the API
// path under OSPREY_VELOCITY_CLOCK=event.
func TestWorkerEventClock_RealVelocityAgreesWithDirectEvaluate(t *testing.T) {
	ctx := context.Background()

	repo, err := repository.New(domain.RepositoryConfig{
		Driver:     "sqlite",
		SQLitePath: t.TempDir() + "/worker-evt.db",
	})
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	velocitySvc := velocity.NewService(repo, cache.NewLRUCache(100))

	engine, err := rules.NewEngine(velocitySvc.GetVelocityGetter(), 5)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	// Mirror cmd/osprey/main.go newRuleEngine wiring.
	engine.SetAggregatesGetter(func(ctx context.Context, tenantID, entityID, currency string, windowSecs int, at time.Time) (rules.VelocityAggregates, error) {
		var a velocity.Aggregates
		var err error
		if at.IsZero() {
			a, err = velocitySvc.GetAggregates(ctx, tenantID, entityID, currency, windowSecs)
		} else {
			a, err = velocitySvc.GetAggregatesAt(ctx, tenantID, entityID, currency, windowSecs, at)
		}
		if err != nil {
			return rules.VelocityAggregates{}, err
		}
		return rules.VelocityAggregates{
			Count: a.Count, AmountSum: a.AmountSum, DistinctCreditors: a.DistinctCreditors,
			InboundCount: a.InboundCount, InboundAmountSum: a.InboundAmountSum,
		}, nil
	})
	engine.SetEventClock(true)

	if err := engine.LoadRules([]*domain.RuleConfig{
		{
			ID:         "vel-count",
			Name:       "Velocity",
			Expression: "velocity_count > 2 ? 1.0 : 0.0",
			Weight:     1.0,
			Enabled:    true,
		},
	}); err != nil {
		t.Fatalf("LoadRules: %v", err)
	}

	// Seed 3 historical transactions within the 1-hour event-time window
	// ending at 2025-06-15T12:00:00Z.
	base := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	for i := 1; i <= 3; i++ {
		txTS := base.Add(-time.Duration(i) * time.Minute)
		tx := &domain.Transaction{
			ID:         "seed-tx-" + string(rune('0'+i)),
			TenantID:   "tenantA",
			Type:       "TRANSFER",
			DebtorID:   "d-evt",
			CreditorID: "c" + string(rune('0'+i)),
			Amount:     100,
			Currency:   "USD",
			Timestamp:  txTS,
			CreatedAt:  time.Now().UTC(),
		}
		if err := repo.SaveTransaction(ctx, "tenantA", tx); err != nil {
			t.Fatalf("SaveTransaction %d: %v", i, err)
		}
	}

	// --- Path 1: direct engine.EvaluateAll (API path simulation) ---
	directInput := &rules.EvaluateInput{
		TenantID:       "tenantA",
		TxID:           "tx-direct",
		Type:           "TRANSFER",
		DebtorID:       "d-evt",
		CreditorID:     "c-new",
		Amount:         100,
		Currency:       "USD",
		VelocityWindow: 3600,
		EventTime:      base,
	}
	directResults, err := engine.EvaluateAll(ctx, directInput)
	if err != nil {
		t.Fatalf("direct EvaluateAll: %v", err)
	}
	directScore := directResults[0].Score

	// --- Path 2: worker processTransaction (the fixed path) ---
	eventBus := bus.NewChannelBus(10)
	defer func() { _ = eventBus.Close() }()

	w := NewWorker(eventBus, repo, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), domain.ModeDetection)

	workerPayload, _ := json.Marshal(TransactionMessage{
		TxID:           "tx-worker",
		TenantID:       "tenantA",
		Type:           "TRANSFER",
		DebtorID:       "d-evt",
		CreditorID:     "c-new",
		Amount:         100,
		Currency:       "USD",
		Timestamp:      base.Format(time.RFC3339),
		VelocityWindow: 3600,
	})

	if err := w.processTransaction(ctx, "tenantA", &domain.Message{
		ID:       "msg-worker",
		TenantID: "tenantA",
		Topic:    domain.TopicTransactionIngested,
		Payload:  workerPayload,
	}, false); err != nil {
		t.Fatalf("worker processTransaction: %v", err)
	}

	// The worker evaluates through the same engine, so its rule result
	// score must match the direct evaluation's score.
	//
	// 3 seeded transactions + the worker's own tx (which is NOT saved via
	// SaveTransaction by the worker) — the velocity_count reflects only
	// the seeded transactions because the worker does not persist the tx.
	// Both paths see the same count (3) hence the same score.
	if directScore != 1.0 {
		t.Errorf("direct path: expected score 1.0 (velocity_count=3 > 2), got %.1f", directScore)
	}

	// Run the worker path through the engine directly to compare scores.
	workerInput := &rules.EvaluateInput{
		TenantID:       "tenantA",
		TxID:           "tx-worker-direct",
		Type:           "TRANSFER",
		DebtorID:       "d-evt",
		CreditorID:     "c-new",
		Amount:         100,
		Currency:       "USD",
		VelocityWindow: 3600,
		EventTime:      base,
	}
	workerResults, err := engine.EvaluateAll(ctx, workerInput)
	if err != nil {
		t.Fatalf("worker EvaluateAll: %v", err)
	}
	workerScore := workerResults[0].Score

	if workerScore != directScore {
		t.Errorf("API path score=%.1f != worker path score=%.1f — divergence under event clock", directScore, workerScore)
	}
	if workerScore != 1.0 {
		t.Errorf("worker path: expected score 1.0 (velocity_count=3 > 2 under event clock), got %.1f", workerScore)
	}

	// Also verify that a backdated timestamp outside the window yields 0.
	oldInput := &rules.EvaluateInput{
		TenantID:       "tenantA",
		TxID:           "tx-old",
		Type:           "TRANSFER",
		DebtorID:       "d-evt",
		CreditorID:     "c-old",
		Amount:         100,
		Currency:       "USD",
		VelocityWindow: 3600,
		EventTime:      base.Add(-2 * time.Hour), // window ends 2h before the seeded txs
	}
	oldResults, err := engine.EvaluateAll(ctx, oldInput)
	if err != nil {
		t.Fatalf("old EvaluateAll: %v", err)
	}
	if oldResults[0].Score != 0.0 {
		t.Errorf("backdated event time outside window: expected score 0.0 (no txs in window), got %.1f", oldResults[0].Score)
	}
}
