// Package worker provides async message processing for the Pro tier.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// Worker processes transactions asynchronously from the EventBus.
type Worker struct {
	bus            domain.EventBus
	repo           domain.Repository
	engine         *rules.Engine
	typologyEngine *rules.TypologyEngine
	processor      *tadp.Processor
	mode           domain.EvaluationMode // detection or compliance

	subscriptions []domain.Subscription
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

// Config holds worker configuration.
type Config struct {
	// TenantIDs is the list of tenants to process (empty = all via wildcard if supported)
	TenantIDs []string

	// WorkerCount is the number of concurrent workers per tenant
	WorkerCount int
}

// NewWorker creates a new async worker.
func NewWorker(bus domain.EventBus, repo domain.Repository, engine *rules.Engine, typologyEngine *rules.TypologyEngine, processor *tadp.Processor, mode domain.EvaluationMode) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		bus:            bus,
		repo:           repo,
		engine:         engine,
		typologyEngine: typologyEngine,
		processor:      processor,
		mode:           mode,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start begins processing messages for the given tenants.
func (w *Worker) Start(cfg Config) error {
	if len(cfg.TenantIDs) == 0 {
		return w.startGlobalWorker()
	}

	for _, tenantID := range cfg.TenantIDs {
		if err := w.startTenantWorker(tenantID); err != nil {
			slog.Error("failed to start worker for tenant",
				"tenant_id", tenantID,
				"error", err,
			)
			continue
		}
	}

	slog.Info("workers started",
		"tenant_count", len(cfg.TenantIDs),
	)

	return nil
}

// startGlobalWorker starts a worker that processes all tenants (for testing/dev).
func (w *Worker) startGlobalWorker() error {
	// Subscribe using a special "global" tenant ID
	// In production, you'd want to subscribe with wildcards or JetStream
	sub, err := w.bus.Subscribe(w.ctx, "_global", domain.TopicTransactionIngested, w.track(w.handleMessage))
	if err != nil {
		return err
	}
	w.subscriptions = append(w.subscriptions, sub)

	slog.Info("global worker started")
	return nil
}

// startTenantWorker starts workers for a specific tenant.
func (w *Worker) startTenantWorker(tenantID string) error {
	// Subscribe to transaction ingested topic
	sub, err := w.bus.Subscribe(w.ctx, tenantID, domain.TopicTransactionIngested, w.track(func(ctx context.Context, msg *domain.Message) error {
		return w.processTransaction(ctx, tenantID, msg, false)
	}))
	if err != nil {
		return err
	}
	w.subscriptions = append(w.subscriptions, sub)

	slog.Info("tenant worker started",
		"tenant_id", tenantID,
		"topic", domain.TopicTransactionIngested,
	)

	return nil
}

// handleMessage handles messages from global subscription.
func (w *Worker) handleMessage(ctx context.Context, msg *domain.Message) error {
	return w.processTransaction(ctx, msg.TenantID, msg, true)
}

// track wraps a handler so Stop()'s wg.Wait() drains in-flight processing before
// main() closes repo/cache/bus. Without this the WaitGroup is never incremented and
// Stop() returns immediately, letting a handler write to closed resources at shutdown.
//
// ponytail: residual narrow race — a message delivered right after cancel()/Unsubscribe()
// could Add() as Wait() observes a zero counter. Acceptable for the Pro async path
// (worst case is one lost in-flight evaluation at shutdown, no corruption/panic).
// Upgrade path: make bus.Unsubscribe() block until the handler goroutine exits.
func (w *Worker) track(h domain.MessageHandler) domain.MessageHandler {
	return func(ctx context.Context, msg *domain.Message) error {
		w.wg.Add(1)
		defer w.wg.Done()
		return h(ctx, msg)
	}
}

// TransactionMessage is the message payload for transaction processing.
type TransactionMessage struct {
	TxID           string         `json:"txId"`
	TenantID       string         `json:"tenantId"`
	TraceID        string         `json:"traceId"`
	Type           string         `json:"type"`
	DebtorID       string         `json:"debtorId"`
	CreditorID     string         `json:"creditorId"`
	Amount         float64        `json:"amount"`
	Currency       string         `json:"currency"`
	VelocityWindow int            `json:"velocityWindow,omitempty"`
	AdditionalData map[string]any `json:"additionalData,omitempty"`
	Enrichment     map[string]any `json:"enrichment,omitempty"`
}

// processTransaction evaluates a transaction through the pipeline.
// Only the global worker trusts the payload tenantId (trustPayloadTenant);
// per-tenant workers keep the subscription tenant.
func (w *Worker) processTransaction(ctx context.Context, tenantID string, msg *domain.Message, trustPayloadTenant bool) error {
	start := time.Now()

	// Compliance-mode entry guard. Capture the typology-set generation atomically
	// with the presence check so a concurrent admin operation that empties the
	// typology set between this guard and the typology evaluation is detected.
	// entryTypologyGen is compared again under a single lock by
	// EvaluateTypologiesIfStable before the decision; a mismatch skips the
	// transaction (returns an error) instead of silently producing a
	// detection-mode decision.
	var entryTypologyGen uint64
	if w.mode == domain.ModeCompliance {
		if w.typologyEngine == nil {
			err := fmt.Errorf("compliance mode requires typologies to be loaded")
			slog.Error("skipping transaction in compliance mode",
				"message_id", msg.ID,
				"tenant_id", tenantID,
				"error", err,
			)
			return err
		}
		gen, count := w.typologyEngine.Snapshot()
		if count == 0 {
			err := fmt.Errorf("compliance mode requires typologies to be loaded")
			slog.Error("skipping transaction in compliance mode",
				"message_id", msg.ID,
				"tenant_id", tenantID,
				"error", err,
			)
			return err
		}
		entryTypologyGen = gen
	}

	// Parse message
	var txMsg TransactionMessage
	if err := json.Unmarshal(msg.Payload, &txMsg); err != nil {
		slog.Error("failed to parse transaction message",
			"message_id", msg.ID,
			"error", err,
		)
		return err
	}

	if trustPayloadTenant && txMsg.TenantID != "" {
		tenantID = txMsg.TenantID
	} else if !trustPayloadTenant && txMsg.TenantID != "" && txMsg.TenantID != tenantID {
		slog.Warn("payload tenantId ignored; subscription tenant is authoritative",
			"subscription_tenant", tenantID,
			"payload_tenant", txMsg.TenantID,
			"message_id", msg.ID,
			"tx_id", txMsg.TxID,
		)
	}

	traceID := txMsg.TraceID
	if traceID == "" {
		traceID = msg.ID
	}

	slog.Debug("processing transaction",
		"tx_id", txMsg.TxID,
		"tenant_id", tenantID,
		"trace_id", traceID,
	)

	// 1. Evaluate rules
	evalInput := &rules.EvaluateInput{
		TenantID:       tenantID,
		TxID:           txMsg.TxID,
		Type:           txMsg.Type,
		DebtorID:       txMsg.DebtorID,
		CreditorID:     txMsg.CreditorID,
		Amount:         txMsg.Amount,
		Currency:       txMsg.Currency,
		VelocityWindow: txMsg.VelocityWindow,
		AdditionalData: txMsg.AdditionalData,
		Enrichment:     txMsg.Enrichment,
	}

	if evalInput.VelocityWindow == 0 {
		evalInput.VelocityWindow = 3600 // Default 1 hour
	}

	ruleResults, err := w.engine.EvaluateAll(ctx, evalInput)
	if err != nil {
		slog.Error("rule evaluation failed",
			"tx_id", txMsg.TxID,
			"error", err,
		)
		return err
	}

	// 2. Evaluate typologies ONLY in Compliance mode. The generation check and
	// the evaluation happen under a single lock on the typology engine, so a
	// concurrent reload that empties the set between the entry guard (which
	// captured entryTypologyGen) and here is detected. On a mismatch the
	// transaction is skipped (returns an error) rather than proceeding with
	// nil TypologyResults, which would silently degrade to detection scoring.
	var typologyResults []domain.TypologyResult
	if w.mode == domain.ModeCompliance && w.typologyEngine != nil {
		results, ok := w.typologyEngine.EvaluateTypologiesIfStable(ruleResults, entryTypologyGen)
		if !ok {
			slog.Warn("typology set changed during compliance evaluation; skipping transaction",
				"tx_id", txMsg.TxID,
				"tenant_id", tenantID,
				"trace_id", traceID,
			)
			return fmt.Errorf("typology configuration changed during evaluation; tx %s skipped", txMsg.TxID)
		}
		typologyResults = results
	}

	// 3. Process decision
	decisionInput := &tadp.DecisionInput{
		TenantID:        tenantID,
		TxID:            txMsg.TxID,
		TraceID:         traceID,
		RuleResults:     ruleResults,
		TypologyResults: typologyResults,
		StartTime:       start,
	}

	evaluation := w.processor.Process(ctx, decisionInput)

	// 4. Save evaluation
	if w.repo != nil {
		if err := w.repo.SaveEvaluation(ctx, tenantID, evaluation); err != nil {
			slog.Error("failed to save evaluation",
				"tx_id", txMsg.TxID,
				"error", err,
			)
			return fmt.Errorf("failed to save evaluation: %w", err)
		}
	}

	// 5. Publish result to decision topic
	resultPayload, err := json.Marshal(evaluation)
	if err != nil {
		return fmt.Errorf("failed to encode evaluation: %w", err)
	}
	if err := w.bus.Publish(ctx, tenantID, domain.TopicDecision, resultPayload); err != nil {
		slog.Error("failed to publish decision",
			"tx_id", txMsg.TxID,
			"error", err,
		)
		return fmt.Errorf("failed to publish decision: %w", err)
	}

	// 6. If alert, publish to alert topic
	if tadp.ShouldAlert(evaluation) {
		if err := w.bus.Publish(ctx, tenantID, domain.TopicAlert, resultPayload); err != nil {
			slog.Error("failed to publish alert",
				"tx_id", txMsg.TxID,
				"error", err,
			)
			return fmt.Errorf("failed to publish alert: %w", err)
		}
	}

	slog.Info("transaction processed",
		"tx_id", txMsg.TxID,
		"tenant_id", tenantID,
		"status", evaluation.Status,
		"score", evaluation.Score,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil
}

// Stop gracefully stops all workers.
func (w *Worker) Stop() error {
	w.cancel()

	// Unsubscribe all
	for _, sub := range w.subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			slog.Error("failed to unsubscribe",
				"topic", sub.Topic(),
				"error", err,
			)
		}
	}
	w.subscriptions = nil

	w.wg.Wait()

	slog.Info("workers stopped")
	return nil
}
