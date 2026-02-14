package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// createTestServer creates a server with engine and processor for testing.
func createTestServer() *Server {
	return createTestServerWithMode(domain.ModeDetection, false)
}

func createTestServerWithMode(mode domain.EvaluationMode, loadTypologies bool) *Server {
	cfg := domain.ServerConfig{
		Host:         "localhost",
		Port:         8080,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	// Create rule engine with test rules (no hardcoded builtin rules)
	engine, _ := rules.NewEngine(nil, 5)

	// Load a test rule that only flags very high amounts (>100000)
	// This ensures normal test amounts don't trigger alerts
	testRule := &domain.RuleConfig{
		ID:         "test-rule-001",
		Name:       "High Value Test Rule",
		Expression: "amount > 100000.0 ? 1.0 : 0.0",
		Weight:     1.0,
		Enabled:    true,
	}
	engine.LoadRule(testRule)

	// Create typology engine
	typologyEngine := rules.NewTypologyEngine()
	if loadTypologies {
		typologyEngine.LoadTypologies([]*domain.Typology{
			{
				ID:             "test-typology-001",
				TenantID:       "*",
				Name:           "Test Typology",
				Version:        "1.0.0",
				AlertThreshold: 0.5,
				Enabled:        true,
				Rules: []domain.TypologyRuleWeight{
					{RuleID: "test-rule-001", Weight: 1.0},
				},
			},
		})
	}

	// Create TADP processor
	processor := tadp.NewProcessor()

	return NewServer(cfg, nil, nil, nil, engine, typologyEngine, processor, "test-v1", mode)
}

func TestEvaluateEndpoint(t *testing.T) {
	server := createTestServer()

	t.Run("SuccessfulEvaluation", func(t *testing.T) {
		reqBody := TransactionRequest{
			Type: "transfer",
			Debtor: PartyInfo{
				ID:        "debtor-001",
				AccountID: "acc-001",
			},
			Creditor: PartyInfo{
				ID:        "creditor-001",
				AccountID: "acc-002",
			},
			Amount: AmountInfo{
				Value:    1000.50,
				Currency: "USD",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp EvaluateResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.EvaluationID == "" {
			t.Error("expected evaluationId in response")
		}
		if resp.TxID == "" {
			t.Error("expected txId in response")
		}
		if resp.Status != domain.StatusNoAlert {
			t.Errorf("expected status NALT, got %s", resp.Status)
		}
		if resp.Metadata.Version != "test-v1" {
			t.Errorf("expected version test-v1, got %s", resp.Metadata.Version)
		}
		if resp.Metadata.TraceID == "" {
			t.Error("expected traceId in metadata")
		}
	})

	t.Run("MissingTenantID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		// No X-Tenant-ID header

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBufferString("not-json"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("MissingType", func(t *testing.T) {
		reqBody := TransactionRequest{
			Debtor:   PartyInfo{ID: "d1", AccountID: "a1"},
			Creditor: PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:   AmountInfo{Value: 100, Currency: "USD"},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("MissingDebtorID", func(t *testing.T) {
		reqBody := TransactionRequest{
			Type:     "transfer",
			Creditor: PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:   AmountInfo{Value: 100, Currency: "USD"},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("InvalidAmount", func(t *testing.T) {
		reqBody := TransactionRequest{
			Type:     "transfer",
			Debtor:   PartyInfo{ID: "d1", AccountID: "a1"},
			Creditor: PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:   AmountInfo{Value: -100, Currency: "USD"},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("ResponseHeaders", func(t *testing.T) {
		reqBody := TransactionRequest{
			Type:     "transfer",
			Debtor:   PartyInfo{ID: "d1", AccountID: "a1"},
			Creditor: PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:   AmountInfo{Value: 100, Currency: "USD"},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Header().Get("X-Request-ID") == "" {
			t.Error("expected X-Request-ID header in response")
		}
		if rr.Header().Get("X-Trace-ID") == "" {
			t.Error("expected X-Trace-ID header in response")
		}
		if rr.Header().Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type: application/json")
		}
	})

	t.Run("ComplianceModeRequiresTypologies", func(t *testing.T) {
		complianceServer := createTestServerWithMode(domain.ModeCompliance, false)

		reqBody := TransactionRequest{
			Type: "transfer",
			Debtor: PartyInfo{
				ID:        "debtor-001",
				AccountID: "acc-001",
			},
			Creditor: PartyInfo{
				ID:        "creditor-001",
				AccountID: "acc-002",
			},
			Amount: AmountInfo{
				Value:    1000.0,
				Currency: "USD",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		complianceServer.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("CreateRuleDoesNotMutateEngineBeforeReload", func(t *testing.T) {
		initialRulesReq := httptest.NewRequest(http.MethodGet, "/rules", nil)
		initialRulesReq.Header.Set("X-Tenant-ID", "tenant-001")
		initialRulesResp := httptest.NewRecorder()
		server.Router().ServeHTTP(initialRulesResp, initialRulesReq)
		if initialRulesResp.Code != http.StatusOK {
			t.Fatalf("failed to fetch initial rules: %d", initialRulesResp.Code)
		}

		rulePayload := map[string]interface{}{
			"id":          "pre-reload-rule",
			"name":        "Pre Reload Rule",
			"description": "Should not be active before reload",
			"expression":  "1 == 1",
			"bands": []map[string]interface{}{
				{"lowerLimit": 1.0, "upperLimit": nil, "subRuleRef": ".fail", "reason": "Always fail"},
				{"lowerLimit": 0.0, "upperLimit": 1.0, "subRuleRef": ".pass", "reason": "Not triggered"},
			},
			"weight":  1.0,
			"enabled": true,
		}

		createBody, _ := json.Marshal(rulePayload)
		createReq := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(createBody))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("X-Tenant-ID", "tenant-001")

		createResp := httptest.NewRecorder()
		server.Router().ServeHTTP(createResp, createReq)
		if createResp.Code != http.StatusCreated {
			t.Fatalf("expected create rule 201, got %d: %s", createResp.Code, createResp.Body.String())
		}

		rulesReq := httptest.NewRequest(http.MethodGet, "/rules", nil)
		rulesReq.Header.Set("X-Tenant-ID", "tenant-001")
		rulesResp := httptest.NewRecorder()
		server.Router().ServeHTTP(rulesResp, rulesReq)
		if rulesResp.Code != http.StatusOK {
			t.Fatalf("failed to fetch rules after create: %d", rulesResp.Code)
		}

		var listed struct {
			Count int `json:"count"`
			Rules []struct {
				ID string `json:"id"`
			} `json:"rules"`
		}
		if err := json.Unmarshal(rulesResp.Body.Bytes(), &listed); err != nil {
			t.Fatalf("failed to parse rules list: %v", err)
		}

		if listed.Count != 1 {
			t.Fatalf("expected loaded rules to remain 1 before reload, got %d", listed.Count)
		}

		for _, r := range listed.Rules {
			if r.ID == "pre-reload-rule" {
				t.Fatalf("rule should not be loaded before reload")
			}
		}

		evalReqBody := TransactionRequest{
			Type: "transfer",
			Debtor: PartyInfo{
				ID:        "debtor-001",
				AccountID: "acc-001",
			},
			Creditor: PartyInfo{
				ID:        "creditor-001",
				AccountID: "acc-002",
			},
			Amount: AmountInfo{
				Value:    100.0,
				Currency: "USD",
			},
		}

		evalBody, _ := json.Marshal(evalReqBody)
		evalReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(evalBody))
		evalReq.Header.Set("Content-Type", "application/json")
		evalReq.Header.Set("X-Tenant-ID", "tenant-001")
		evalResp := httptest.NewRecorder()
		server.Router().ServeHTTP(evalResp, evalReq)
		if evalResp.Code != http.StatusOK {
			t.Fatalf("expected evaluation to succeed, got %d: %s", evalResp.Code, evalResp.Body.String())
		}

		var evalResult EvaluateResponse
		if err := json.Unmarshal(evalResp.Body.Bytes(), &evalResult); err != nil {
			t.Fatalf("failed to parse evaluation response: %v", err)
		}
		if evalResult.Status != domain.StatusNoAlert {
			t.Fatalf("expected NALT without reload, got %s", evalResult.Status)
		}
	})
}

func TestHealthEndpoint(t *testing.T) {
	server := createTestServer()

	t.Run("HealthCheck", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)

		if resp["status"] != "healthy" {
			t.Errorf("expected status 'healthy', got '%s'", resp["status"])
		}
		if resp["version"] != "test-v1" {
			t.Errorf("expected version 'test-v1', got '%s'", resp["version"])
		}
		if resp["mode"] != "detection" {
			t.Errorf("expected mode 'detection', got '%s'", resp["mode"])
		}
	})

	t.Run("ReadyCheck", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("ComplianceHealthIsDegradedWithoutTypologies", func(t *testing.T) {
		complianceServer := createTestServerWithMode(domain.ModeCompliance, false)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()
		complianceServer.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode health response: %v", err)
		}

		if resp["status"] != "degraded" {
			t.Fatalf("expected degraded health, got %v", resp["status"])
		}
	})

	t.Run("ComplianceReadyIsUnavailableWithoutTypologies", func(t *testing.T) {
		complianceServer := createTestServerWithMode(domain.ModeCompliance, false)

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()
		complianceServer.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode ready response: %v", err)
		}

		if resp["ready"] != "false" {
			t.Fatalf("expected ready=false, got %q", resp["ready"])
		}
	})
}

func TestMiddleware(t *testing.T) {
	t.Run("TenantMiddlewareExtractsID", func(t *testing.T) {
		var capturedTenantID string

		handler := TenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedTenantID = GetTenantID(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Tenant-ID", "my-tenant-123")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if capturedTenantID != "my-tenant-123" {
			t.Errorf("expected tenant ID 'my-tenant-123', got '%s'", capturedTenantID)
		}
	})

	t.Run("TracingMiddlewareSetsRequestID", func(t *testing.T) {
		var capturedRequestID string

		handler := TracingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Access context value directly (GetRequestID was removed as dead code)
			if v, ok := r.Context().Value(RequestIDKey).(string); ok {
				capturedRequestID = v
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if capturedRequestID == "" {
			t.Error("expected request ID to be set")
		}

		if rr.Header().Get("X-Request-ID") == "" {
			t.Error("expected X-Request-ID response header")
		}
	})

	t.Run("RecoverMiddlewareHandlesPanic", func(t *testing.T) {
		handler := RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		// Should not panic
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})
}
