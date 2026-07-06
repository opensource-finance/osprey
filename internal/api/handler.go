package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// Handler holds dependencies for API handlers.
type Handler struct {
	repo           domain.Repository
	cache          domain.Cache
	bus            domain.EventBus
	engine         *rules.Engine
	typologyEngine *rules.TypologyEngine
	processor      *tadp.Processor
	version        string
	mode           domain.EvaluationMode // detection or compliance
}

const maxJSONBodyBytes = 1 << 20

// NewHandler creates a new API handler.
func NewHandler(repo domain.Repository, cache domain.Cache, bus domain.EventBus, engine *rules.Engine, typologyEngine *rules.TypologyEngine, processor *tadp.Processor, version string, mode domain.EvaluationMode) *Handler {
	return &Handler{
		repo:           repo,
		cache:          cache,
		bus:            bus,
		engine:         engine,
		typologyEngine: typologyEngine,
		processor:      processor,
		version:        version,
		mode:           mode,
	}
}

// TransactionRequest is the request body for POST /evaluate.
type TransactionRequest struct {
	ID        string         `json:"id,omitempty"`
	Type      string         `json:"type"`
	Debtor    PartyInfo      `json:"debtor"`
	Creditor  PartyInfo      `json:"creditor"`
	Amount    AmountInfo     `json:"amount"`
	Timestamp string         `json:"timestamp,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	// Enrichment carries externally-computed scores/flags (ml_score, sanctions_hit,
	// ring_risk, ...). Caller-asserted: Osprey does not verify these values.
	Enrichment map[string]any `json:"enrichment,omitempty"`
}

// PartyInfo represents a debtor or creditor.
type PartyInfo struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
}

// AmountInfo represents the transaction amount.
type AmountInfo struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

// EvaluateResponse is the response for POST /evaluate.
type EvaluateResponse struct {
	EvaluationID string   `json:"evaluationId"`
	TxID         string   `json:"txId,omitempty"`
	Status       string   `json:"status"`
	Score        float64  `json:"score"`
	Reasons      []string `json:"reasons,omitempty"`
	Metadata     struct {
		TraceID  string `json:"traceId"`
		IngestMs int64  `json:"ingestMs"`
		TotalMs  int64  `json:"totalMs"`
		Version  string `json:"version"`
	} `json:"metadata"`
}

// Evaluate handles POST /evaluate requests.
func (h *Handler) Evaluate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	tenantID := GetTenantID(ctx)
	traceID := GetTraceID(ctx)

	if h.mode == domain.ModeCompliance && !h.hasLoadedTypologies() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "compliance mode requires typologies to be loaded",
		})
		return
	}

	// Parse request
	var req TransactionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	// Validate required fields
	req.Type = strings.ToUpper(strings.TrimSpace(req.Type))
	if req.Type == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "type is required",
		})
		return
	}
	if req.Debtor.ID == "" || req.Creditor.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "debtor.id and creditor.id are required",
		})
		return
	}
	if req.Amount.Value <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "amount.value must be positive",
		})
		return
	}
	req.Amount.Currency = strings.ToUpper(strings.TrimSpace(req.Amount.Currency))
	if req.Amount.Currency == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "amount.currency is required",
		})
		return
	}

	txID := strings.TrimSpace(req.ID)
	if txID == "" {
		txID = uuid.New().String()
	}

	now := time.Now().UTC()
	txTimestamp := now
	if strings.TrimSpace(req.Timestamp) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.Timestamp))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "timestamp must be RFC3339",
			})
			return
		}
		txTimestamp = parsed.UTC()
	}

	ingestMs := time.Since(start).Milliseconds()

	// Create transaction record
	tx := &domain.Transaction{
		ID:              txID,
		TenantID:        tenantID,
		Type:            req.Type,
		DebtorID:        req.Debtor.ID,
		DebtorAccountID: req.Debtor.AccountID,
		CreditorID:      req.Creditor.ID,
		CreditorAcctID:  req.Creditor.AccountID,
		Amount:          req.Amount.Value,
		Currency:        req.Amount.Currency,
		Timestamp:       txTimestamp,
		CreatedAt:       now,
		Metadata:        req.Metadata,
		Enrichment:      req.Enrichment,
	}

	// Save transaction if repository is available
	if h.repo != nil {
		if err := h.repo.SaveTransaction(ctx, tenantID, tx); errors.Is(err, repository.ErrDuplicateTransaction) {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "transaction id already exists",
			})
			return
		} else if err != nil {
			slog.Error("failed to save transaction", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to persist transaction",
			})
			return
		}
	}

	// Synchronous Evaluation
	// Detection mode: Rules → Weighted Score → Alert
	// Compliance mode: Rules → Typologies → FATF patterns → Alert

	// 1. Prepare input
	evalInput := &rules.EvaluateInput{
		TenantID:       tenantID,
		TxID:           txID,
		Type:           tx.Type,
		DebtorID:       tx.DebtorID,
		CreditorID:     tx.CreditorID,
		Amount:         tx.Amount,
		Currency:       tx.Currency,
		VelocityWindow: 3600, // Default 1 hour window
		AdditionalData: tx.Metadata,
		Enrichment:     tx.Enrichment,
	}

	// 2. Evaluate rules
	ruleResults, err := h.engine.EvaluateAll(ctx, evalInput)
	if err != nil {
		slog.Error("rule evaluation failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "rule evaluation failed",
		})
		return
	}

	// 3. Evaluate typologies ONLY in Compliance mode
	var typologyResults []domain.TypologyResult
	if h.mode == domain.ModeCompliance && h.typologyEngine != nil && h.typologyEngine.TypologyCount() > 0 {
		typologyResults = h.typologyEngine.EvaluateTypologies(ruleResults)
	}

	// 4. Process decision
	decisionInput := &tadp.DecisionInput{
		TenantID:        tenantID,
		TxID:            txID,
		TraceID:         traceID,
		RuleResults:     ruleResults,
		TypologyResults: typologyResults,
		StartTime:       start,
	}

	evaluation := h.processor.Process(ctx, decisionInput)

	// 5. Save evaluation
	if h.repo != nil {
		if err := h.repo.SaveEvaluation(ctx, tenantID, evaluation); err != nil {
			slog.Error("failed to save evaluation", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to persist evaluation",
			})
			return
		}
	}

	totalMs := time.Since(start).Milliseconds()

	// 6. Respond
	resp := EvaluateResponse{
		EvaluationID: evaluation.ID,
		TxID:         txID,
		Status:       evaluation.Status,
		Score:        evaluation.Score,
		Reasons:      tadp.GetReasons(evaluation),
	}
	resp.Metadata.TraceID = traceID
	resp.Metadata.IngestMs = ingestMs
	resp.Metadata.TotalMs = totalMs
	resp.Metadata.Version = h.version

	writeJSON(w, http.StatusOK, resp)
}

// Health returns server health status.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	status := "healthy"

	// Check repository health
	if h.repo != nil {
		if err := h.repo.Ping(r.Context()); err != nil {
			status = "degraded"
		}
	}

	// Check cache health
	if h.cache != nil {
		if err := h.cache.Ping(r.Context()); err != nil {
			status = "degraded"
		}
	}

	if h.mode == domain.ModeCompliance && !h.hasLoadedTypologies() {
		status = "degraded"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  status,
		"version": h.version,
		"mode":    string(h.mode),
	})
}

// Ready returns whether the server is ready to accept traffic.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.mode == domain.ModeCompliance && !h.hasLoadedTypologies() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"ready": "false",
			"error": "compliance mode requires typologies to be loaded",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"ready": "true",
	})
}

// GetEvaluation retrieves an evaluation by ID.
func (h *Handler) GetEvaluation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := GetTenantID(ctx)
	evalID := chi.URLParam(r, "id")

	if evalID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "evaluation id is required",
		})
		return
	}

	if h.repo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "repository not available",
		})
		return
	}

	eval, err := h.repo.GetEvaluation(ctx, tenantID, evalID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "evaluation not found",
			})
			return
		}
		slog.Error("failed to get evaluation", "id", evalID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to get evaluation",
		})
		return
	}

	writeJSON(w, http.StatusOK, eval)
}

// GetTransaction retrieves a transaction by ID.
func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := GetTenantID(ctx)
	txID := chi.URLParam(r, "id")

	if txID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "transaction id is required",
		})
		return
	}

	if h.repo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "repository not available",
		})
		return
	}

	tx, err := h.repo.GetTransaction(ctx, tenantID, txID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "transaction not found",
			})
			return
		}
		slog.Error("failed to get transaction", "id", txID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to get transaction",
		})
		return
	}

	writeJSON(w, http.StatusOK, tx)
}

// ListVariables returns the CEL variables available to rule expressions, from the
// canonical rules.Catalog. Studio's rule builder reads this instead of hardcoding.
func (h *Handler) ListVariables(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"variables": rules.Catalog,
		"count":     len(rules.Catalog),
	})
}

// ListRules returns the rules currently active in the evaluation engine.
func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	loadedRules := h.engine.GetLoadedRules()

	writeJSON(w, http.StatusOK, map[string]any{
		"rules":  loadedRules,
		"count":  len(loadedRules),
		"source": "active-engine",
	})
}

// GetRule retrieves an active rule by ID from the evaluation engine.
func (h *Handler) GetRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "id")

	if ruleID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "rule id is required",
		})
		return
	}

	for _, rule := range h.engine.GetLoadedRules() {
		if rule.ID == ruleID {
			writeJSON(w, http.StatusOK, rule)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": "rule not found",
	})
}

// CreateRuleRequest is the request body for creating a rule.
type CreateRuleRequest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Expression  string            `json:"expression"`
	Bands       []domain.RuleBand `json:"bands"`
	Weight      float64           `json:"weight"`
	Enabled     bool              `json:"enabled"`
}

// CreateRule creates a new rule and saves it to the database.
// Rules are saved globally (tenant_id = "*") so they apply to all tenants.
// The active rule engine is reloaded after persistence succeeds.
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.requireRepository(w) {
		return
	}

	var req CreateRuleRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	req.Expression = strings.TrimSpace(req.Expression)

	if !validateRuleRequest(w, &req) {
		return
	}

	// Create rule config (global tenant)
	ruleConfig := &domain.RuleConfig{
		ID:          req.ID,
		TenantID:    domain.GlobalTenantID,
		Name:        req.Name,
		Description: req.Description,
		Version:     "1.0.0",
		Expression:  req.Expression,
		Bands:       req.Bands,
		Weight:      req.Weight,
		Enabled:     req.Enabled,
	}

	// Validate CEL expression without mutating loaded engine rules.
	if err := h.engine.ValidateRule(ruleConfig); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid CEL expression: " + err.Error(),
		})
		return
	}

	// Persist to repository (global tenant ID)
	if err := h.repo.SaveRuleConfig(ctx, domain.GlobalTenantID, ruleConfig); err != nil {
		slog.Error("failed to save rule config", "id", ruleConfig.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to save rule",
		})
		return
	}
	loadedCount, err := h.reloadRulesFromRepository(ctx)
	if err != nil {
		slog.Error("failed to reload rule engine", "id", ruleConfig.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "rule saved but engine reload failed; check server logs",
		})
		return
	}
	slog.Info("rule engine reloaded after rule save", "count", loadedCount)

	slog.Info("rule created", "id", ruleConfig.ID, "name", ruleConfig.Name)
	writeJSON(w, http.StatusCreated, map[string]any{
		"rule":    ruleConfig,
		"message": "rule saved and loaded",
	})
}

// UpdateRule updates an existing rule by ID and reloads the engine.
// Rules are stored globally (tenant_id = "*"); the URL id is authoritative.
func (h *Handler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ruleID := strings.TrimSpace(chi.URLParam(r, "id"))

	if !h.requireRepository(w) {
		return
	}
	if ruleID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "rule id is required",
		})
		return
	}

	var req CreateRuleRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.ID = ruleID
	req.Name = strings.TrimSpace(req.Name)
	req.Expression = strings.TrimSpace(req.Expression)

	if !validateRuleRequest(w, &req) {
		return
	}

	ruleConfig := &domain.RuleConfig{
		ID:          ruleID,
		TenantID:    domain.GlobalTenantID,
		Name:        req.Name,
		Description: req.Description,
		Version:     "1.0.0",
		Expression:  req.Expression,
		Bands:       req.Bands,
		Weight:      req.Weight,
		Enabled:     req.Enabled,
	}

	// Validate CEL expression without mutating loaded engine rules.
	if err := h.engine.ValidateRule(ruleConfig); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid CEL expression: " + err.Error(),
		})
		return
	}

	if err := h.repo.SaveRuleConfig(ctx, domain.GlobalTenantID, ruleConfig); err != nil {
		slog.Error("failed to update rule config", "id", ruleID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to update rule",
		})
		return
	}
	loadedCount, err := h.reloadRulesFromRepository(ctx)
	if err != nil {
		slog.Error("failed to reload rule engine", "id", ruleID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "rule updated but engine reload failed; check server logs",
		})
		return
	}
	slog.Info("rule engine reloaded after rule update", "count", loadedCount)

	slog.Info("rule updated", "id", ruleID)
	writeJSON(w, http.StatusOK, map[string]any{
		"rule":    ruleConfig,
		"message": "rule updated and loaded",
	})
}

// DeleteRule disables a rule and reloads the engine. A rule referenced by a
// loaded typology cannot be deleted (409) so typology evaluation stays valid.
func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ruleID := strings.TrimSpace(chi.URLParam(r, "id"))

	if !h.requireRepository(w) {
		return
	}
	if ruleID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "rule id is required",
		})
		return
	}

	// Referential integrity: refuse to delete a rule a loaded typology depends on.
	if h.typologyEngine != nil {
		for _, t := range h.typologyEngine.GetLoadedTypologies() {
			for _, tr := range t.Rules {
				if tr.RuleID == ruleID {
					writeJSON(w, http.StatusConflict, map[string]string{
						"error": fmt.Sprintf("rule %q is referenced by typology %q; remove it from the typology first", ruleID, t.ID),
					})
					return
				}
			}
		}
	}

	if err := h.repo.DeleteRuleConfig(ctx, domain.GlobalTenantID, ruleID); errors.Is(err, repository.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "rule not found",
		})
		return
	} else if err != nil {
		slog.Error("failed to delete rule", "id", ruleID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to delete rule",
		})
		return
	}

	loadedCount, err := h.reloadRulesFromRepository(ctx)
	if err != nil {
		slog.Error("failed to reload rules after delete", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "rule deleted but engine reload failed; check server logs",
		})
		return
	}
	slog.Info("rules reloaded after delete", "count", loadedCount)

	slog.Info("rule deleted", "id", ruleID)
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "rule deleted and engine reloaded",
	})
}

func validateRuleRequest(w http.ResponseWriter, req *CreateRuleRequest) bool {
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "id is required",
		})
		return false
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "name is required",
		})
		return false
	}
	if req.Expression == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "expression is required",
		})
		return false
	}
	if req.Weight < 0 || req.Weight > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "weight must be between 0 and 1",
		})
		return false
	}

	for i, band := range req.Bands {
		req.Bands[i].SubRuleRef = strings.TrimSpace(band.SubRuleRef)
		req.Bands[i].Reason = strings.TrimSpace(band.Reason)

		if req.Bands[i].SubRuleRef == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "bands.subRuleRef is required",
			})
			return false
		}
		if req.Bands[i].Reason == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "bands.reason is required",
			})
			return false
		}
		if band.LowerLimit != nil && band.UpperLimit != nil && *band.LowerLimit >= *band.UpperLimit {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "bands.lowerLimit must be less than bands.upperLimit",
			})
			return false
		}
	}

	return true
}

// ReloadRules reloads all rules from the database into the engine.
// CreateRule already reloads automatically; this endpoint is for manual recovery.
func (h *Handler) ReloadRules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.repo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "repository not available",
		})
		return
	}

	loadedCount, err := h.reloadRulesFromRepository(ctx)
	if err != nil {
		slog.Error("failed to reload rules into engine", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to reload rules; check server logs",
		})
		return
	}

	slog.Info("rules reloaded from database", "count", loadedCount)
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "rules reloaded successfully",
		"count":   loadedCount,
	})
}

func (h *Handler) reloadRulesFromRepository(ctx context.Context) (int, error) {
	dbRules, err := h.repo.ListRuleConfigs(ctx, domain.GlobalTenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to load rules from database: %w", err)
	}
	if err := h.engine.ReloadRules(dbRules); err != nil {
		return 0, err
	}
	return len(dbRules), nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) requireRepository(w http.ResponseWriter) bool {
	if h.repo != nil {
		return true
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{
		"error": "repository not available",
	})
	return false
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		writeJSONDecodeError(w, err)
		return false
	}

	var extra struct{}
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeJSONDecodeError(w, err)
		return false
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error": "request body must contain a single JSON object",
	})
	return false
}

func writeJSONDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
			"error": "request body too large",
		})
		return
	}

	if errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "request body is required",
		})
		return
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error": "invalid JSON request body: " + err.Error(),
	})
}

func (h *Handler) hasLoadedTypologies() bool {
	return h.typologyEngine != nil && h.typologyEngine.TypologyCount() > 0
}

// ============================================================================
// TYPOLOGY HANDLERS
// ============================================================================

// CreateTypologyRequest is the request body for creating a typology.
type CreateTypologyRequest struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description,omitempty"`
	Rules          []domain.TypologyRuleWeight `json:"rules"`
	AlertThreshold float64                     `json:"alertThreshold"`
	Enabled        bool                        `json:"enabled"`
}

// ListTypologies returns the typologies currently active in the typology engine.
func (h *Handler) ListTypologies(w http.ResponseWriter, r *http.Request) {
	if h.typologyEngine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "typology engine not available",
		})
		return
	}

	typologies := h.typologyEngine.GetLoadedTypologies()

	writeJSON(w, http.StatusOK, map[string]any{
		"typologies": typologies,
		"count":      len(typologies),
		"source":     "active-engine",
	})
}

// GetTypology retrieves an active typology by ID from the typology engine.
func (h *Handler) GetTypology(w http.ResponseWriter, r *http.Request) {
	typologyID := chi.URLParam(r, "id")

	if typologyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "typology id is required",
		})
		return
	}

	if h.typologyEngine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "typology engine not available",
		})
		return
	}

	for _, t := range h.typologyEngine.GetLoadedTypologies() {
		if t.ID == typologyID {
			writeJSON(w, http.StatusOK, t)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": "typology not found",
	})
}

// CreateTypology creates a new typology and saves it to the database.
func (h *Handler) CreateTypology(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.requireRepository(w) {
		return
	}

	var req CreateTypologyRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)

	if !h.validateTypologyRequest(w, &req, true) {
		return
	}

	// Create typology config (global tenant)
	typology := &domain.Typology{
		ID:             req.ID,
		TenantID:       domain.GlobalTenantID,
		Name:           req.Name,
		Description:    req.Description,
		Version:        "1.0.0",
		Rules:          req.Rules,
		AlertThreshold: req.AlertThreshold,
		Enabled:        req.Enabled,
	}

	// Persist to repository
	if err := h.repo.SaveTypology(ctx, domain.GlobalTenantID, typology); err != nil {
		slog.Error("failed to save typology", "id", typology.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to save typology",
		})
		return
	}
	loadedCount, err := h.reloadTypologiesFromRepository(ctx)
	if err != nil {
		slog.Error("failed to reload typology engine", "id", typology.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "typology saved but engine reload failed; check server logs",
		})
		return
	}
	slog.Info("typology engine reloaded after typology save", "count", loadedCount)

	slog.Info("typology created", "id", typology.ID, "name", typology.Name)
	writeJSON(w, http.StatusCreated, map[string]any{
		"typology": typology,
		"message":  "typology saved and loaded",
	})
}

// UpdateTypology updates an existing typology.
func (h *Handler) UpdateTypology(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	typologyID := chi.URLParam(r, "id")

	if !h.requireRepository(w) {
		return
	}

	if typologyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "typology id is required",
		})
		return
	}

	var req CreateTypologyRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)

	if !h.validateTypologyRequest(w, &req, false) {
		return
	}

	// Update typology
	typology := &domain.Typology{
		ID:             typologyID,
		TenantID:       domain.GlobalTenantID,
		Name:           req.Name,
		Description:    req.Description,
		Version:        "1.0.0",
		Rules:          req.Rules,
		AlertThreshold: req.AlertThreshold,
		Enabled:        req.Enabled,
	}

	if err := h.repo.SaveTypology(ctx, domain.GlobalTenantID, typology); err != nil {
		slog.Error("failed to update typology", "id", typologyID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to update typology",
		})
		return
	}
	loadedCount, err := h.reloadTypologiesFromRepository(ctx)
	if err != nil {
		slog.Error("failed to reload typology engine", "id", typologyID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "typology updated but engine reload failed; check server logs",
		})
		return
	}
	slog.Info("typology engine reloaded after typology update", "count", loadedCount)

	slog.Info("typology updated", "id", typologyID)
	writeJSON(w, http.StatusOK, map[string]any{
		"typology": typology,
		"message":  "typology updated and loaded",
	})
}

func (h *Handler) validateTypologyRequest(w http.ResponseWriter, req *CreateTypologyRequest, requireID bool) bool {
	if requireID && req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "id is required",
		})
		return false
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "name is required",
		})
		return false
	}
	if len(req.Rules) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "at least one rule is required",
		})
		return false
	}

	ruleIDSet := make(map[string]bool)
	if h.engine != nil {
		loadedRules := h.engine.GetLoadedRules()
		ruleIDSet = make(map[string]bool, len(loadedRules))
		for _, r := range loadedRules {
			ruleIDSet[r.ID] = true
		}
	}

	var totalWeight float64
	for i, rule := range req.Rules {
		ruleID := strings.TrimSpace(rule.RuleID)
		if ruleID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "ruleId cannot be empty",
			})
			return false
		}
		if !ruleIDSet[ruleID] {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("ruleId %q does not exist in rule engine", ruleID),
			})
			return false
		}
		req.Rules[i].RuleID = ruleID
		if rule.Weight < 0 || rule.Weight > 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "rule weight must be between 0 and 1",
			})
			return false
		}
		totalWeight += rule.Weight
	}

	if totalWeight < 0.99 || totalWeight > 1.01 {
		slog.Warn("typology weights do not sum to 1.0",
			"typology_id", req.ID,
			"total_weight", totalWeight,
		)
	}

	if req.AlertThreshold <= 0 || req.AlertThreshold > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "alertThreshold must be between 0 (exclusive) and 1",
		})
		return false
	}

	return true
}

// DeleteTypology deletes a typology and auto-reloads the engine.
func (h *Handler) DeleteTypology(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	typologyID := chi.URLParam(r, "id")

	if !h.requireRepository(w) {
		return
	}

	if typologyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "typology id is required",
		})
		return
	}

	if err := h.repo.DeleteTypology(ctx, domain.GlobalTenantID, typologyID); errors.Is(err, repository.ErrNotFound) {
		slog.Error("failed to delete typology", "id", typologyID, "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "typology not found",
		})
		return
	} else if err != nil {
		slog.Error("failed to delete typology", "id", typologyID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to delete typology",
		})
		return
	}

	loadedCount, err := h.reloadTypologiesFromRepository(ctx)
	if err != nil {
		slog.Error("failed to reload typologies after delete", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "typology deleted but engine reload failed; check server logs",
		})
		return
	}
	slog.Info("typologies reloaded after delete", "count", loadedCount)

	slog.Info("typology deleted", "id", typologyID)
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Typology deleted and engine reloaded.",
	})
}

// ReloadTypologies reloads all typologies from the database into the engine.
func (h *Handler) ReloadTypologies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.repo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "repository not available",
		})
		return
	}

	if h.typologyEngine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "typology engine not available",
		})
		return
	}

	loadedCount, err := h.reloadTypologiesFromRepository(ctx)
	if err != nil {
		slog.Error("failed to reload typologies into engine", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to reload typologies; check server logs",
		})
		return
	}

	slog.Info("typologies reloaded from database", "count", loadedCount)
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "typologies reloaded successfully",
		"count":   loadedCount,
	})
}

func (h *Handler) reloadTypologiesFromRepository(ctx context.Context) (int, error) {
	if h.typologyEngine == nil {
		return 0, fmt.Errorf("typology engine not available")
	}
	dbTypologies, err := h.repo.ListTypologies(ctx, domain.GlobalTenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to load typologies from database: %w", err)
	}
	h.typologyEngine.ReloadTypologies(dbTypologies)
	return len(dbTypologies), nil
}
