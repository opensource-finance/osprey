package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/cache"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
	"github.com/opensource-finance/osprey/internal/velocity"
)

// TestProcessTransactionRejectsMissingCurrency is the regression test for the
// async-path validation gap. A bus message with a missing or empty currency
// used to reach the rule engine with Currency == "". The velocity layer's
// sameCurrency legacy fallback then matched every row, so velocity_amount_sum
// and velocity_inbound_amount_sum added raw amounts across every currency in
// the window. The worker now rejects the message with "currency is required",
// mirroring POST /evaluate's "amount.currency is required" 400, so the
// velocity layer's empty-currency fallback is no longer reachable from a
// producer on the bus.
func TestProcessTransactionRejectsMissingCurrency(t *testing.T) {
	cases := []struct {
		name     string
		currency string
	}{
		{"omitted", ""}, // JSON `{}` with no currency key unmarshals to ""
		{"empty", ""},
		{"whitespace", "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getter := &capturingAggregatesGetter{}
			engine, _ := rules.NewEngine(nil, 5)
			engine.SetAggregatesGetter(getter.get)
			if err := engine.LoadRules([]*domain.RuleConfig{
				{ID: "vel-sum", Name: "Velocity Sum", Expression: "velocity_amount_sum >= 150.0 ? 1.0 : 0.0", Weight: 1.0, Enabled: true},
			}); err != nil {
				t.Fatalf("LoadRules: %v", err)
			}

			repo := &capturingRepository{}
			busCap := &capturingBus{}
			w := NewWorker(busCap, repo, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), domain.ModeDetection)

			txMsg := TransactionMessage{
				TxID:       "tx-no-ccy",
				TenantID:   "tenantA",
				Type:       "transfer",
				DebtorID:   "d1",
				CreditorID: "c1",
				Amount:     100.0,
				Currency:   tc.currency,
			}
			payload, err := json.Marshal(txMsg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			err = w.processTransaction(context.Background(), "tenantA", &domain.Message{
				ID: "msg-no-ccy", TenantID: "tenantA",
				Topic: domain.TopicTransactionIngested, Payload: payload,
			}, false)
			if err == nil || !strings.Contains(err.Error(), "currency is required") {
				t.Fatalf("expected 'currency is required' error, got %v", err)
			}

			// The velocity layer's empty-currency fallback must not be
			// reachable: the aggregates getter is never called when currency
			// is empty, so the mixed-currency sum cannot be produced.
			if _, calls := getter.snapshot(); calls != 0 {
				t.Errorf("aggregates getter called %d times; velocity layer reached with empty currency", calls)
			}
			if len(repo.savedEvals()) != 0 {
				t.Errorf("expected no SaveEvaluation on rejection, got %d", len(repo.savedEvals()))
			}
			if len(busCap.published()) != 0 {
				t.Errorf("expected no Publish on rejection, got %d publishes", len(busCap.published()))
			}
		})
	}
}

// TestProcessTransactionRejectsMissingType mirrors POST /evaluate's "type is
// required" check on the async path. The worker previously normalized type
// but did not require it.
func TestProcessTransactionRejectsMissingType(t *testing.T) {
	cases := []struct {
		name      string
		typeValue string
	}{
		{"omitted", ""},
		{"empty", ""},
		{"whitespace", "  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, repo, busCap := newCapturingWorker(t, tadp.NewProcessor(), noAlertRule())

			txMsg := TransactionMessage{
				TxID:       "tx-no-type",
				TenantID:   "tenantA",
				Type:       tc.typeValue,
				DebtorID:   "d1",
				CreditorID: "c1",
				Amount:     100.0,
				Currency:   "USD",
			}
			payload, err := json.Marshal(txMsg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			err = w.processTransaction(context.Background(), "tenantA", &domain.Message{
				ID: "msg-no-type", TenantID: "tenantA",
				Topic: domain.TopicTransactionIngested, Payload: payload,
			}, false)
			if err == nil || !strings.Contains(err.Error(), "type is required") {
				t.Fatalf("expected 'type is required' error, got %v", err)
			}
			if len(repo.savedEvals()) != 0 {
				t.Errorf("expected no SaveEvaluation on rejection, got %d", len(repo.savedEvals()))
			}
			if len(busCap.published()) != 0 {
				t.Errorf("expected no Publish on rejection, got %d publishes", len(busCap.published()))
			}
		})
	}
}

// TestProcessTransactionRejectsNonPositiveAmount mirrors POST /evaluate's
// "amount.value must be positive" check on the async path.
func TestProcessTransactionRejectsNonPositiveAmount(t *testing.T) {
	cases := []struct {
		name   string
		amount float64
	}{
		{"zero", 0.0},
		{"negative", -100.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, repo, busCap := newCapturingWorker(t, tadp.NewProcessor(), noAlertRule())

			txMsg := TransactionMessage{
				TxID:       "tx-no-amt",
				TenantID:   "tenantA",
				Type:       "transfer",
				DebtorID:   "d1",
				CreditorID: "c1",
				Amount:     tc.amount,
				Currency:   "USD",
			}
			payload, err := json.Marshal(txMsg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			err = w.processTransaction(context.Background(), "tenantA", &domain.Message{
				ID: "msg-no-amt", TenantID: "tenantA",
				Topic: domain.TopicTransactionIngested, Payload: payload,
			}, false)
			if err == nil || !strings.Contains(err.Error(), "amount must be positive") {
				t.Fatalf("expected 'amount must be positive' error, got %v", err)
			}
			if len(repo.savedEvals()) != 0 {
				t.Errorf("expected no SaveEvaluation on rejection, got %d", len(repo.savedEvals()))
			}
			if len(busCap.published()) != 0 {
				t.Errorf("expected no Publish on rejection, got %d publishes", len(busCap.published()))
			}
		})
	}
}

// TestProcessTransactionCurrencyValidationMatchesSyncPath reproduces the
// currency-parity bug end-to-end with a real SQLite repository, the real
// velocity service, and a rule keyed on velocity_amount_sum. Entity E holds
// 100 USD received and 100 EUR sent. A currency-scoped sum is 100 (under the
// 150 threshold), but the velocity layer's empty-currency fallback returns
// the mixed 200 sum, which would trip the threshold to a false-positive ALRT.
// The worker must reject the missing-currency message (so no decision is
// published), matching what POST /evaluate does with HTTP 400. A control
// message with currency "EUR" must evaluate to NALT (currency-scoped 100).
func TestProcessTransactionCurrencyValidationMatchesSyncPath(t *testing.T) {
	ctx := context.Background()

	repo, err := repository.New(domain.RepositoryConfig{
		Driver:     "sqlite",
		SQLitePath: t.TempDir() + "/ccy-parity.db",
	})
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	velocitySvc := velocity.NewService(repo, cache.NewLRUCache(100))

	engine, err := rules.NewEngine(nil, 5)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	// Mirror cmd/osprey/main.go newRuleEngine wiring.
	engine.SetAggregatesGetter(func(ctx context.Context, tenantID, entityID, currency string, windowSecs int, at time.Time) (rules.VelocityAggregates, error) {
		var a velocity.Aggregates
		var gerr error
		if at.IsZero() {
			a, gerr = velocitySvc.GetAggregates(ctx, tenantID, entityID, currency, windowSecs)
		} else {
			a, gerr = velocitySvc.GetAggregatesAt(ctx, tenantID, entityID, currency, windowSecs, at)
		}
		if gerr != nil {
			return rules.VelocityAggregates{}, gerr
		}
		return rules.VelocityAggregates{
			Count: a.Count, AmountSum: a.AmountSum, DistinctCreditors: a.DistinctCreditors,
			InboundCount: a.InboundCount, InboundAmountSum: a.InboundAmountSum,
		}, nil
	})

	if err := engine.LoadRules([]*domain.RuleConfig{
		{ID: "vel-sum", Name: "Velocity Sum", Expression: "velocity_amount_sum >= 150.0 ? 1.0 : 0.0", Weight: 1.0, Enabled: true},
	}); err != nil {
		t.Fatalf("LoadRules: %v", err)
	}

	// Seed: 100 USD received by E, 100 EUR sent by E.
	now := time.Now().UTC()
	seed := func(id, from, to, currency string, amount float64) {
		tx := &domain.Transaction{
			ID: id, Type: "TRANSFER", DebtorID: from, CreditorID: to,
			Amount: amount, Currency: currency, Timestamp: now, CreatedAt: now,
		}
		if err := repo.SaveTransaction(ctx, "tenantA", tx); err != nil {
			t.Fatalf("SaveTransaction %s: %v", id, err)
		}
	}
	seed("in-usd", "x", "E", "USD", 100)
	seed("out-eur", "E", "y", "EUR", 100)

	// Sanity: a currency-scoped sum is 100, the empty-currency fallback is 200.
	aggUSD, err := velocitySvc.GetAggregates(ctx, "tenantA", "E", "USD", 3600)
	if err != nil {
		t.Fatalf("GetAggregates USD: %v", err)
	}
	aggEUR, err := velocitySvc.GetAggregates(ctx, "tenantA", "E", "EUR", 3600)
	if err != nil {
		t.Fatalf("GetAggregates EUR: %v", err)
	}
	aggAll, err := velocitySvc.GetAggregates(ctx, "tenantA", "E", "", 3600)
	if err != nil {
		t.Fatalf("GetAggregates all: %v", err)
	}
	if aggUSD.AmountSum != 100 || aggEUR.AmountSum != 100 || aggAll.AmountSum != 200 {
		t.Fatalf("seed sanity: USD=%.0f EUR=%.0f ALL=%.0f, want 100/100/200", aggUSD.AmountSum, aggEUR.AmountSum, aggAll.AmountSum)
	}

	capRepo := &capturingRepository{}
	capBus := &capturingBus{}
	w := NewWorker(capBus, capRepo, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), domain.ModeDetection)

	// Case 1: missing currency. Must be rejected, with no evaluation saved or
	// published. Before the fix this produced a false-positive ALRT driven by
	// the mixed 200 sum.
	noCcyPayload, _ := json.Marshal(TransactionMessage{
		TxID: "tx-mixed", TenantID: "tenantA", Type: "transfer",
		DebtorID: "E", CreditorID: "z", Amount: 10,
		VelocityWindow: 3600, // Currency intentionally omitted.
	})
	err = w.processTransaction(ctx, "tenantA", &domain.Message{
		ID: "msg-mixed", TenantID: "tenantA",
		Topic: domain.TopicTransactionIngested, Payload: noCcyPayload,
	}, false)
	if err == nil || !strings.Contains(err.Error(), "currency is required") {
		t.Fatalf("missing currency: expected 'currency is required', got %v", err)
	}
	if len(capRepo.savedEvals()) != 0 {
		t.Errorf("missing currency: expected no SaveEvaluation, got %d", len(capRepo.savedEvals()))
	}
	if len(capBus.published()) != 0 {
		t.Errorf("missing currency: expected no Publish, got %d", len(capBus.published()))
	}

	// Case 2: explicit currency "EUR". Currency-scoped sum is 100, under the
	// 150 threshold, so the rule does not fire and the decision is NALT.
	eurPayload, _ := json.Marshal(TransactionMessage{
		TxID: "tx-eur", TenantID: "tenantA", Type: "transfer",
		DebtorID: "E", CreditorID: "z", Amount: 10, Currency: "EUR",
		VelocityWindow: 3600,
	})
	if err := w.processTransaction(ctx, "tenantA", &domain.Message{
		ID: "msg-eur", TenantID: "tenantA",
		Topic: domain.TopicTransactionIngested, Payload: eurPayload,
	}, false); err != nil {
		t.Fatalf("EUR control: processTransaction: %v", err)
	}
	saved := capRepo.savedEvals()
	if len(saved) != 1 {
		t.Fatalf("EUR control: expected 1 SaveEvaluation, got %d", len(saved))
	}
	if saved[0].Eval.Status != domain.StatusNoAlert {
		t.Errorf("EUR control: expected NALT (currency-scoped sum 100 < 150), got %s", saved[0].Eval.Status)
	}
	if got := countPublishes(capBus.published(), "tenantA", domain.TopicDecision); got != 1 {
		t.Errorf("EUR control: expected 1 decision publish, got %d", got)
	}
	if got := countPublishes(capBus.published(), "tenantA", domain.TopicAlert); got != 0 {
		t.Errorf("EUR control: expected 0 alert publishes (NALT), got %d", got)
	}
}
