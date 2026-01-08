// Package worker provides async message processing for the Pro tier.
package worker

import (
	"context"
	"encoding/json"
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
func NewWorker(bus domain.EventBus, repo domain.Repository, engine *rules.Engine, typologyEngine *rules.TypologyEngine, processor *tadp.Processor) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		bus:            bus,
		repo:           repo,
		engine:         engine,
		typologyEngine: typologyEngine,
		processor:      processor,
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
	sub, err := w.bus.Subscribe(w.ctx, "_global", domain.TopicTransactionIngested, w.handleMessage)
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
	sub, err := w.bus.Subscribe(w.ctx, tenantID, domain.TopicTransactionIngested, func(ctx context.Context, msg *domain.Message) error {
		return w.processTransaction(ctx, tenantID, msg)
	})
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
	return w.processTransaction(ctx, msg.TenantID, msg)
}

// TransactionMessage is the message payload for transaction processing.
type TransactionMessage struct {
	TxID            string         `json:"txId"`
	TenantID        string         `json:"tenantId"`
	TraceID         string         `json:"traceId"`
	Type            string         `json:"type"`
	DebtorID        string         `json:"debtorId"`
	CreditorID      string         `json:"creditorId"`
	Amount          float64        `json:"amount"`
	Currency        string         `json:"currency"`
	VelocityWindow  int            `json:"velocityWindow,omitempty"`
	AdditionalData  map[string]any `json:"additionalData,omitempty"`
}

// processTransaction evaluates a transaction through the pipeline.
func (w *Worker) processTransaction(ctx context.Context, tenantID string, msg *domain.Message) error {
	start := time.Now()

	// Parse message
	var txMsg TransactionMessage
	if err := json.Unmarshal(msg.Payload, &txMsg); err != nil {
		slog.Error("failed to parse transaction message",
			"message_id", msg.ID,
			"error", err,
		)
		return err
	}

	// Use message tenant if provided
	if txMsg.TenantID != "" {
		tenantID = txMsg.TenantID
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

	// 2. Evaluate typologies based on rule results
	var typologyResults []domain.TypologyResult
	if w.typologyEngine != nil && w.typologyEngine.TypologyCount() > 0 {
		typologyResults = w.typologyEngine.EvaluateTypologies(ruleResults)
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
		}
	}

	// 5. Publish result to decision topic
	resultPayload, _ := json.Marshal(evaluation)
	if err := w.bus.Publish(ctx, tenantID, domain.TopicDecision, resultPayload); err != nil {
		slog.Error("failed to publish decision",
			"tx_id", txMsg.TxID,
			"error", err,
		)
	}

	// 6. If alert, publish to alert topic
	if tadp.ShouldAlert(evaluation) {
		if err := w.bus.Publish(ctx, tenantID, domain.TopicAlert, resultPayload); err != nil {
			slog.Error("failed to publish alert",
				"tx_id", txMsg.TxID,
				"error", err,
			)
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

// Stats returns worker statistics.
type Stats struct {
	SubscriptionCount int      `json:"subscriptionCount"`
	Topics            []string `json:"topics"`
}

// GetStats returns current worker statistics.
func (w *Worker) GetStats() Stats {
	topics := make([]string, len(w.subscriptions))
	for i, sub := range w.subscriptions {
		topics[i] = sub.Topic()
	}
	return Stats{
		SubscriptionCount: len(w.subscriptions),
		Topics:            topics,
	}
}
