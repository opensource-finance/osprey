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

	// Create TADP processor
	processor := tadp.NewProcessor()

	return NewServer(cfg, nil, nil, nil, engine, typologyEngine, processor, "test-v1")
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

		var resp map[string]string
		json.Unmarshal(rr.Body.Bytes(), &resp)

		if resp["status"] != "healthy" {
			t.Errorf("expected status 'healthy', got '%s'", resp["status"])
		}
		if resp["version"] != "test-v1" {
			t.Errorf("expected version 'test-v1', got '%s'", resp["version"])
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
