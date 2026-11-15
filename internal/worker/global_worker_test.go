package worker

import (
	"context"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/bus"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// TestGlobalWorker_ReceivesAllTenantPublishes is the regression test for the
// async worker bug: when the worker is started with an empty TenantIDs list
// (the exact configuration main() used to build when OSPREY_TENANTS was
// unset), it must still receive transactions published for real tenants.
//
// Before the fix, the global worker subscribed as the exact tenant "_global",
// which both buses keyed by exact tenant, so a publish for "tenantA" reached
// only "tenantA" subscribers and the global worker logged success while
// processing zero transactions. After the fix, the global worker subscribes
// with domain.GlobalTenantID ("*"), which both buses treat as a wildcard, so
// the publish for "tenantA" is delivered and the decision is routed back to
// "tenantA" via the payload tenant.
func TestGlobalWorker_ReceivesAllTenantPublishes(t *testing.T) {
	eventBus := bus.NewChannelBus(100)
	defer func() { _ = eventBus.Close() }()

	engine, _ := rules.NewEngine(nil, 5)
	_ = engine.LoadRules(noAlertRule())

	repo := &capturingRepository{}
	w := NewWorker(eventBus, repo, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), domain.ModeDetection)

	// Empty TenantIDs selects the global (wildcard) worker. This is the
	// shape main() used to pass when OSPREY_TENANTS was unset.
	if err := w.Start(Config{TenantIDs: []string{}}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = w.Stop() }()

	decA := subscribeChan(t, eventBus, "tenanta", domain.TopicDecision)
	decB := subscribeChan(t, eventBus, "tenantb", domain.TopicDecision)

	time.Sleep(50 * time.Millisecond) // let subscriptions register

	// Publish an ingested transaction for tenanta. The global worker must
	// receive it through the wildcard subscription, evaluate it, and publish
	// the decision back on tenanta's decision topic.
	payloadA := marshalTxMsg(t, TransactionMessage{
		TxID:       "tx-global-a",
		TenantID:   "tenanta",
		Type:       "transfer",
		DebtorID:   "d1",
		CreditorID: "c1",
		Amount:     500.0,
		Currency:   "USD",
	})
	if err := eventBus.Publish(context.Background(), "tenanta", domain.TopicTransactionIngested, payloadA); err != nil {
		t.Fatalf("publish tenanta: %v", err)
	}

	// A second publish for a different tenant proves the wildcard reaches
	// every tenant, not just one.
	payloadB := marshalTxMsg(t, TransactionMessage{
		TxID:       "tx-global-b",
		TenantID:   "tenantb",
		Type:       "transfer",
		DebtorID:   "d2",
		CreditorID: "c2",
		Amount:     500.0,
		Currency:   "USD",
	})
	if err := eventBus.Publish(context.Background(), "tenantb", domain.TopicTransactionIngested, payloadB); err != nil {
		t.Fatalf("publish tenantb: %v", err)
	}

	// Both decisions must land on the right tenant's topic. Before the fix,
	// these receives timed out because the global worker never got the
	// ingested publishes.
	recvMsg(t, decA, 2*time.Second, "tenanta:decision")
	recvMsg(t, decB, 2*time.Second, "tenantb:decision")

	// The global worker trusts the payload tenant, so the evaluations must
	// be attributed to the real tenants, never to the wildcard marker.
	saved := repo.savedEvals()
	if len(saved) != 2 {
		t.Fatalf("expected 2 SaveEvaluation calls, got %d", len(saved))
	}
	tenants := map[string]bool{}
	for _, e := range saved {
		if e.TenantID == domain.GlobalTenantID || e.TenantID == "" {
			t.Errorf("evaluation attributed to %q; global worker must recover the real tenant from the payload", e.TenantID)
		}
		tenants[e.TenantID] = true
	}
	if !tenants["tenanta"] || !tenants["tenantb"] {
		t.Errorf("expected evaluations for tenanta and tenantb, got %v", tenants)
	}
}

// TestGlobalWorker_DoesNotCrossRouteToOtherTenants confirms the global worker
// publishes decisions on the real (payload) tenant, so a tenant that did not
// publish does not receive a decision.
func TestGlobalWorker_DoesNotCrossRouteToOtherTenants(t *testing.T) {
	eventBus := bus.NewChannelBus(100)
	defer func() { _ = eventBus.Close() }()

	engine, _ := rules.NewEngine(nil, 5)
	_ = engine.LoadRules(noAlertRule())

	repo := &capturingRepository{}
	w := NewWorker(eventBus, repo, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), domain.ModeDetection)

	if err := w.Start(Config{TenantIDs: []string{}}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = w.Stop() }()

	decA := subscribeChan(t, eventBus, "tenanta", domain.TopicDecision)
	decB := subscribeChan(t, eventBus, "tenantb", domain.TopicDecision)

	time.Sleep(50 * time.Millisecond)

	payloadA := marshalTxMsg(t, TransactionMessage{
		TxID:       "tx-only-a",
		TenantID:   "tenanta",
		Type:       "transfer",
		DebtorID:   "d1",
		CreditorID: "c1",
		Amount:     500.0,
		Currency:   "USD",
	})
	if err := eventBus.Publish(context.Background(), "tenanta", domain.TopicTransactionIngested, payloadA); err != nil {
		t.Fatalf("publish: %v", err)
	}

	recvMsg(t, decA, 2*time.Second, "tenanta:decision")
	assertNoMsg(t, decB, "tenantb:decision")
}
