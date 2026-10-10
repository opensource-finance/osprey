package worker

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/bus"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

func TestVelocityBypass_NegativeWindowSkipsVelocity(t *testing.T) {
	eventBus := bus.NewChannelBus(10)
	defer func() { _ = eventBus.Close() }()

	var getterCalls atomic.Int32
	velocityGetter := func(ctx context.Context, tenantID, entityID string, windowSecs int) (int64, error) {
		getterCalls.Add(1)
		return 100, nil
	}
	engine, _ := rules.NewEngine(velocityGetter, 2)
	_ = engine.LoadRules([]*domain.RuleConfig{
		{
			ID:         "velocity-rule",
			Name:       "Velocity Check",
			Expression: "velocity_count > 5 ? 1.0 : 0.0",
			Weight:     1.0,
			Enabled:    true,
		},
	})

	processor := &tadp.Processor{AlertThreshold: 0.5, UseWeightedScoring: true}
	w := NewWorker(eventBus, nil, engine, rules.NewTypologyEngine(), processor, domain.ModeDetection)

	runCase := func(t *testing.T, txID string, velocityWindow int) (string, float64) {
		var received atomic.Bool
		var evalPayload []byte

		sub, _ := eventBus.Subscribe(context.Background(), "tenantA", domain.TopicDecision, func(ctx context.Context, msg *domain.Message) error {
			var eval domain.Evaluation
			if json.Unmarshal(msg.Payload, &eval) == nil && eval.TxID == txID {
				evalPayload = msg.Payload
				received.Store(true)
			}
			return nil
		})
		defer func() { _ = sub.Unsubscribe() }()

		time.Sleep(20 * time.Millisecond)

		payload, _ := json.Marshal(TransactionMessage{
			TxID:           txID,
			TenantID:       "tenantA",
			Type:           "transfer",
			DebtorID:       "d1",
			CreditorID:     "c1",
			Amount:         500.0,
			Currency:       "USD",
			VelocityWindow: velocityWindow,
		})

		err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
			ID:       "msg-" + txID,
			TenantID: "tenantA",
			Topic:    domain.TopicTransactionIngested,
			Payload:  payload,
		}, false)
		if err != nil {
			t.Fatalf("processTransaction failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		if !received.Load() {
			t.Fatalf("no decision published for %s", txID)
		}

		var eval domain.Evaluation
		if err := json.Unmarshal(evalPayload, &eval); err != nil {
			t.Fatalf("failed to unmarshal evaluation: %v", err)
		}
		return eval.Status, eval.Score
	}

	// Normal case: velocityWindow=3600 -> velocity lookup runs -> count=100 -> ALRT
	statusNormal, scoreNormal := runCase(t, "tx-normal", 3600)
	if statusNormal != domain.StatusAlert {
		t.Errorf("normal: expected ALRT, got %s (score=%.1f)", statusNormal, scoreNormal)
	}

	// Bypass case: velocityWindow=-1 would pass the == 0 guard and reach the
	// engine's <= 0 opt-out path, skipping the velocity lookup so velocity_count
	// stays 0 and the rule does not fire -> NALT. After the fix (<= 0 guard) the
	// negative value is defaulted to 3600 so the lookup runs -> ALRT.
	statusBypass, scoreBypass := runCase(t, "tx-bypass", -1)
	if statusBypass != domain.StatusAlert {
		t.Errorf("bypass: expected ALRT, got %s (score=%.1f) — velocity lookup was skipped because velocityWindow=-1 passed the == 0 guard",
			statusBypass, scoreBypass)
	}
}
