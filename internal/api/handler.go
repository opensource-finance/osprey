package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/opensource-finance/osprey/internal/domain"
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
}

// NewHandler creates a new API handler.
func NewHandler(repo domain.Repository, cache domain.Cache, bus domain.EventBus, engine *rules.Engine, typologyEngine *rules.TypologyEngine, processor *tadp.Processor, version string) *Handler {
	return &Handler{
		repo:           repo,
		cache:          cache,
		bus:            bus,
		engine:         engine,
		typologyEngine: typologyEngine,
		processor:      processor,
		version:        version,
	}
}

// TransactionRequest is the request body for POST /evaluate.
type TransactionRequest struct {
	Type     string                 `json:"type"`
	Debtor   PartyInfo              `json:"debtor"`
	Creditor PartyInfo              `json:"creditor"`
	Amount   AmountInfo             `json:"amount"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
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

	// Parse request
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON request body",
		})
		return
	}

	// Validate required fields
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

	// Generate IDs
	txID := uuid.New().String()

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
		Timestamp:       time.Now().UTC(),
		CreatedAt:       time.Now().UTC(),
		Metadata:        req.Metadata,
	}

	// Save transaction if repository is available
	if h.repo != nil {
		if err := h.repo.SaveTransaction(ctx, tenantID, tx); err != nil {
			slog.Error("failed to save transaction", "error", err)
			// Continue even if save fails? For now, yes, to prioritize evaluation.
		}
	}

	// Synchronous Evaluation (Community Tier / Direct Mode)
	// We execute rules directly via the engine + processor

	// 1. Prepare input
	evalInput := &rules.EvaluateInput{
		TenantID:       tenantID,
		TxID:           txID,
		Type:           tx.Type,
		DebtorID:       tx.DebtorID,
		CreditorID:     tx.CreditorID,
		Amount:         tx.Amount,
		Currency:       tx.Currency,
		VelocityWindow: 3600, // Default 1 hour window?
		AdditionalData: tx.Metadata,
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

	// 3. Evaluate typologies based on rule results
	var typologyResults []domain.TypologyResult
	if h.typologyEngine != nil && h.typologyEngine.TypologyCount() > 0 {
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

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  status,
		"version": h.version,
	})
}

// Ready returns whether the server is ready to accept traffic.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
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
		slog.Error("failed to get evaluation", "id", evalID, "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "evaluation not found",
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
		slog.Error("failed to get transaction", "id", txID, "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "transaction not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, tx)
}

// ListRules returns all loaded rules from the engine.
// Rules are loaded from the database at startup and can be reloaded via POST /rules/reload.
func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	// Return rules currently loaded in the engine (sourced from database)
	loadedRules := h.engine.GetLoadedRules()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rules":  loadedRules,
		"count":  len(loadedRules),
		"source": "database",
	})
}

// GetRule retrieves a rule by ID from the loaded engine rules.
func (h *Handler) GetRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "id")

	if ruleID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "rule id is required",
		})
		return
	}

	// Check rules loaded in the engine (from database)
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
// After saving, call POST /rules/reload to hot-reload into the engine.
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON request body",
		})
		return
	}

	// Validate
	if req.ID == "" || req.Name == "" || req.Expression == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "id, name, and expression are required",
		})
		return
	}

	// Create rule config (global tenant)
	ruleConfig := &domain.RuleConfig{
		ID:          req.ID,
		TenantID:    GlobalTenantID,
		Name:        req.Name,
		Description: req.Description,
		Version:     "1.0.0",
		Expression:  req.Expression,
		Bands:       req.Bands,
		Weight:      req.Weight,
		Enabled:     req.Enabled,
	}

	// Validate CEL expression by attempting to load
	if err := h.engine.LoadRule(ruleConfig); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid CEL expression: " + err.Error(),
		})
		return
	}

	// Persist to repository (global tenant ID)
	if h.repo != nil {
		if err := h.repo.SaveRuleConfig(ctx, GlobalTenantID, ruleConfig); err != nil {
			slog.Error("failed to save rule config", "id", ruleConfig.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to save rule",
			})
			return
		}
	}

	slog.Info("rule created", "id", ruleConfig.ID, "name", ruleConfig.Name)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"rule":    ruleConfig,
		"message": "Rule created. Call POST /rules/reload to apply changes.",
	})
}

// GlobalTenantID is used for rules that apply to all tenants.
const GlobalTenantID = "*"

// ReloadRules reloads all rules from the database into the engine.
// This enables hot-reloading without server restart.
func (h *Handler) ReloadRules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.repo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "repository not available",
		})
		return
	}

	// Load rules from database (global rules)
	dbRules, err := h.repo.ListRuleConfigs(ctx, GlobalTenantID)
	if err != nil {
		slog.Error("failed to list rules from database", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to load rules from database",
		})
		return
	}

	// Reload into engine
	if err := h.engine.ReloadRules(dbRules); err != nil {
		slog.Error("failed to reload rules into engine", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to reload rules: " + err.Error(),
		})
		return
	}

	slog.Info("rules reloaded from database", "count", len(dbRules))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "rules reloaded successfully",
		"count":   len(dbRules),
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
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

// ListTypologies returns all loaded typologies.
func (h *Handler) ListTypologies(w http.ResponseWriter, r *http.Request) {
	if h.typologyEngine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "typology engine not available",
		})
		return
	}

	typologies := h.typologyEngine.GetLoadedTypologies()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"typologies": typologies,
		"count":      len(typologies),
		"source":     "database",
	})
}

// GetTypology retrieves a typology by ID.
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

	// Check typologies loaded in the engine
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

	var req CreateTypologyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON request body",
		})
		return
	}

	// Validate required fields
	if req.ID == "" || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "id and name are required",
		})
		return
	}

	if len(req.Rules) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "at least one rule is required",
		})
		return
	}

	// Validate rules exist in engine and weights are valid
	loadedRules := h.engine.GetLoadedRules()
	ruleIDSet := make(map[string]bool, len(loadedRules))
	for _, r := range loadedRules {
		ruleIDSet[r.ID] = true
	}

	var totalWeight float64
	for _, rule := range req.Rules {
		if rule.RuleID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "rule_id cannot be empty",
			})
			return
		}
		if !ruleIDSet[rule.RuleID] {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("rule_id '%s' does not exist in rule engine", rule.RuleID),
			})
			return
		}
		if rule.Weight < 0 || rule.Weight > 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "rule weight must be between 0 and 1",
			})
			return
		}
		totalWeight += rule.Weight
	}

	// Warn if weights don't sum to approximately 1.0 (allow 0.01 tolerance)
	if totalWeight < 0.99 || totalWeight > 1.01 {
		slog.Warn("typology weights do not sum to 1.0",
			"typology_id", req.ID,
			"total_weight", totalWeight,
		)
	}

	// Validate threshold - must be > 0 to avoid triggering on every transaction
	if req.AlertThreshold <= 0 || req.AlertThreshold > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "alertThreshold must be between 0 (exclusive) and 1",
		})
		return
	}

	// Create typology config (global tenant)
	typology := &domain.Typology{
		ID:             req.ID,
		TenantID:       GlobalTenantID,
		Name:           req.Name,
		Description:    req.Description,
		Version:        "1.0.0",
		Rules:          req.Rules,
		AlertThreshold: req.AlertThreshold,
		Enabled:        req.Enabled,
	}

	// Persist to repository
	if h.repo != nil {
		if err := h.repo.SaveTypology(ctx, GlobalTenantID, typology); err != nil {
			slog.Error("failed to save typology", "id", typology.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to save typology",
			})
			return
		}
	}

	slog.Info("typology created", "id", typology.ID, "name", typology.Name)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"typology": typology,
		"message":  "Typology created. Call POST /typologies/reload to apply changes.",
	})
}

// UpdateTypology updates an existing typology.
func (h *Handler) UpdateTypology(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	typologyID := chi.URLParam(r, "id")

	if typologyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "typology id is required",
		})
		return
	}

	var req CreateTypologyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON request body",
		})
		return
	}

	// Validate rules
	for _, rule := range req.Rules {
		if rule.RuleID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "rule_id cannot be empty",
			})
			return
		}
		if rule.Weight < 0 || rule.Weight > 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "rule weight must be between 0 and 1",
			})
			return
		}
	}

	// Update typology
	typology := &domain.Typology{
		ID:             typologyID,
		TenantID:       GlobalTenantID,
		Name:           req.Name,
		Description:    req.Description,
		Version:        "1.0.0",
		Rules:          req.Rules,
		AlertThreshold: req.AlertThreshold,
		Enabled:        req.Enabled,
	}

	if h.repo != nil {
		if err := h.repo.SaveTypology(ctx, GlobalTenantID, typology); err != nil {
			slog.Error("failed to update typology", "id", typologyID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to update typology",
			})
			return
		}
	}

	slog.Info("typology updated", "id", typologyID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"typology": typology,
		"message":  "Typology updated. Call POST /typologies/reload to apply changes.",
	})
}

// DeleteTypology deletes a typology and auto-reloads the engine.
func (h *Handler) DeleteTypology(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	typologyID := chi.URLParam(r, "id")

	if typologyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "typology id is required",
		})
		return
	}

	if h.repo != nil {
		if err := h.repo.DeleteTypology(ctx, GlobalTenantID, typologyID); err != nil {
			slog.Error("failed to delete typology", "id", typologyID, "error", err)
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "typology not found",
			})
			return
		}

		// Auto-reload typology engine after delete
		if h.typologyEngine != nil {
			dbTypologies, err := h.repo.ListTypologies(ctx, GlobalTenantID)
			if err != nil {
				slog.Error("failed to reload typologies after delete", "error", err)
			} else {
				h.typologyEngine.ReloadTypologies(dbTypologies)
				slog.Info("typologies auto-reloaded after delete", "count", len(dbTypologies))
			}
		}
	}

	slog.Info("typology deleted", "id", typologyID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
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

	// Load typologies from database (global)
	dbTypologies, err := h.repo.ListTypologies(ctx, GlobalTenantID)
	if err != nil {
		slog.Error("failed to list typologies from database", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to load typologies from database",
		})
		return
	}

	// Reload into engine
	h.typologyEngine.ReloadTypologies(dbTypologies)

	slog.Info("typologies reloaded from database", "count", len(dbTypologies))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "typologies reloaded successfully",
		"count":   len(dbTypologies),
	})
}
