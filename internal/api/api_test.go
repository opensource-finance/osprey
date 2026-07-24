package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
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
	_ = engine.LoadRule(testRule)

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

type failingRepository struct {
	domain.Repository
	saveTransactionErr error
	saveEvaluationErr  error
	getTransactionErr  error
	getEvaluationErr   error
	deleteTypologyErr  error
}

const testAdminToken = "test-admin-token"

func setAdminAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
}

func (r *failingRepository) GetTransaction(_ context.Context, _ string, _ string) (*domain.Transaction, error) {
	if r.getTransactionErr != nil {
		return nil, r.getTransactionErr
	}
	return nil, repository.ErrNotFound
}

func (r *failingRepository) GetEvaluation(_ context.Context, _ string, _ string) (*domain.Evaluation, error) {
	if r.getEvaluationErr != nil {
		return nil, r.getEvaluationErr
	}
	return nil, repository.ErrNotFound
}

func (r *failingRepository) SaveTransaction(_ context.Context, _ string, _ *domain.Transaction) error {
	return r.saveTransactionErr
}

func (r *failingRepository) SaveEvaluation(_ context.Context, _ string, _ *domain.Evaluation) error {
	return r.saveEvaluationErr
}

func (r *failingRepository) DeleteTypology(_ context.Context, _ string, _ string) error {
	return r.deleteTypologyErr
}

func createTestServerWithRepository(repo domain.Repository) *Server {
	return createTestServerWithRepositoryAndAdminToken(repo, "")
}

func createTestServerWithRepositoryAndAdminToken(repo domain.Repository, adminToken string) *Server {
	if adminToken == "" {
		adminToken = testAdminToken
	}
	cfg := domain.ServerConfig{
		Host:         "localhost",
		Port:         8080,
		ReadTimeout:  30,
		WriteTimeout: 30,
		AdminToken:   adminToken,
	}
	engine, _ := rules.NewEngine(nil, 5)
	processor := tadp.NewProcessor()
	return NewServer(cfg, repo, nil, nil, engine, rules.NewTypologyEngine(), processor, "test-v1", domain.ModeDetection)
}

func createPersistentTestServer(t *testing.T) (*Server, func()) {
	t.Helper()
	engine, _ := rules.NewEngine(nil, 5)
	return createPersistentTestServerWithEngine(t, engine)
}

func createPersistentTestServerWithEngine(t *testing.T, engine *rules.Engine) (*Server, func()) {
	return createPersistentTestServerWithEngineAndAdminToken(t, engine, "")
}

func createPersistentTestServerWithEngineAndAdminToken(t *testing.T, engine *rules.Engine, adminToken string) (*Server, func()) {
	t.Helper()
	if adminToken == "" {
		adminToken = testAdminToken
	}

	dbFile, err := os.CreateTemp("", "osprey-api-*.db")
	if err != nil {
		t.Fatalf("failed to create temp database: %v", err)
	}
	dbPath := dbFile.Name()
	if err := dbFile.Close(); err != nil {
		t.Fatalf("failed to close temp database: %v", err)
	}

	repo, err := repository.New(domain.RepositoryConfig{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	})
	if err != nil {
		_ = os.Remove(dbPath)
		t.Fatalf("failed to create repository: %v", err)
	}

	cleanup := func() {
		_ = repo.Close()
		_ = os.Remove(dbPath)
		_ = os.Remove(dbPath + "-shm")
		_ = os.Remove(dbPath + "-wal")
	}

	processor := tadp.NewProcessor()
	server := NewServer(
		domain.ServerConfig{Host: "localhost", Port: 8080, ReadTimeout: 30, WriteTimeout: 30, AdminToken: adminToken},
		repo,
		nil,
		nil,
		engine,
		rules.NewTypologyEngine(),
		processor,
		"test-v1",
		domain.ModeDetection,
	)

	return server, cleanup
}

//nolint:gocognit // sequential subtests sharing fixture state; complexity is scenario count, not branching logic
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

	t.Run("RejectsUnknownJSONFields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBufferString(`{
			"type": "transfer",
			"debtor": {"id": "d1", "accountId": "a1"},
			"creditor": {"id": "c1", "accountId": "a2"},
			"amount": {"value": 100, "currency": "USD"},
			"unexpected": true
		}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
		}
		var resp map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse error response: %v", err)
		}
		if !strings.Contains(resp["error"], `unknown field "unexpected"`) {
			t.Fatalf("expected unknown field error, got %q", resp["error"])
		}
	})

	t.Run("RejectsTrailingJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBufferString(`{
			"type": "transfer",
			"debtor": {"id": "d1", "accountId": "a1"},
			"creditor": {"id": "c1", "accountId": "a2"},
			"amount": {"value": 100, "currency": "USD"}
		}
		{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("single JSON object")) {
			t.Fatalf("expected single-object error, got %s", rr.Body.String())
		}
	})

	t.Run("RejectsOversizedJSON", func(t *testing.T) {
		var body bytes.Buffer
		body.WriteString(`{"type":"transfer","debtor":{"id":"d1","accountId":"a1"},"creditor":{"id":"c1","accountId":"a2"},"amount":{"value":100,"currency":"USD"},"metadata":{"pad":"`)
		body.Write(bytes.Repeat([]byte("a"), maxJSONBodyBytes))
		body.WriteString(`"}}`)

		req := httptest.NewRequest(http.MethodPost, "/evaluate", &body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected status 413, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("InvalidTimestamp", func(t *testing.T) {
		reqBody := TransactionRequest{
			Type:      "transfer",
			Debtor:    PartyInfo{ID: "d1", AccountID: "a1"},
			Creditor:  PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:    AmountInfo{Value: 100, Currency: "USD"},
			Timestamp: "not-a-timestamp",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rr.Code)
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

	t.Run("MissingCurrency", func(t *testing.T) {
		reqBody := TransactionRequest{
			Type:     "transfer",
			Debtor:   PartyInfo{ID: "d1", AccountID: "a1"},
			Creditor: PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:   AmountInfo{Value: 100},
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

	t.Run("NormalizesTypeAndCurrency", func(t *testing.T) {
		cfg := domain.ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30,
			WriteTimeout: 30,
		}
		engine, _ := rules.NewEngine(nil, 5)
		err := engine.LoadRule(&domain.RuleConfig{
			ID:         "normalized-transfer",
			Name:       "Normalized Transfer",
			Expression: "tx_type == \"TRANSFER\" && currency == \"USD\" ? 1.0 : 0.0",
			Bands: []domain.RuleBand{
				{LowerLimit: new(1.0), SubRuleRef: ".fail", Reason: "Normalized type and currency matched"},
				{LowerLimit: new(0.0), UpperLimit: new(1.0), SubRuleRef: ".pass", Reason: "Not matched"},
			},
			Weight:  1.0,
			Enabled: true,
		})
		if err != nil {
			t.Fatalf("failed to load normalization rule: %v", err)
		}
		normalizationServer := NewServer(cfg, nil, nil, nil, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), "test-v1", domain.ModeDetection)

		reqBody := TransactionRequest{
			Type:     "transfer",
			Debtor:   PartyInfo{ID: "d1", AccountID: "a1"},
			Creditor: PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:   AmountInfo{Value: 100, Currency: "usd"},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		normalizationServer.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp EvaluateResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Status != domain.StatusAlert {
			t.Fatalf("expected normalized lower-case request to alert, got %s", resp.Status)
		}
	})

	t.Run("PreservesClientTransactionIDAndTimestamp", func(t *testing.T) {
		server, cleanup := createPersistentTestServer(t)
		defer cleanup()

		reqBody := TransactionRequest{
			ID:        "client-tx-001",
			Type:      "transfer",
			Debtor:    PartyInfo{ID: "d1", AccountID: "a1"},
			Creditor:  PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:    AmountInfo{Value: 100, Currency: "usd"},
			Timestamp: "2026-05-25T09:15:30Z",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")

		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp EvaluateResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.TxID != "client-tx-001" {
			t.Fatalf("expected response txId to preserve client ID, got %q", resp.TxID)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/transactions/client-tx-001", nil)
		getReq.Header.Set("X-Tenant-ID", "tenant-001")
		getResp := httptest.NewRecorder()
		server.Router().ServeHTTP(getResp, getReq)

		if getResp.Code != http.StatusOK {
			t.Fatalf("expected get transaction 200, got %d: %s", getResp.Code, getResp.Body.String())
		}

		var tx domain.Transaction
		if err := json.Unmarshal(getResp.Body.Bytes(), &tx); err != nil {
			t.Fatalf("failed to parse transaction response: %v", err)
		}
		if tx.ID != "client-tx-001" {
			t.Fatalf("expected stored transaction ID to be preserved, got %q", tx.ID)
		}
		wantTimestamp := time.Date(2026, 5, 25, 9, 15, 30, 0, time.UTC)
		if !tx.Timestamp.Equal(wantTimestamp) {
			t.Fatalf("expected timestamp %s, got %s", wantTimestamp, tx.Timestamp)
		}
	})

	t.Run("DuplicateClientTransactionIDReturnsConflict", func(t *testing.T) {
		server, cleanup := createPersistentTestServer(t)
		defer cleanup()

		reqBody := TransactionRequest{
			ID:       "duplicate-client-tx",
			Type:     "transfer",
			Debtor:   PartyInfo{ID: "d1", AccountID: "a1"},
			Creditor: PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:   AmountInfo{Value: 100, Currency: "USD"},
		}
		body, _ := json.Marshal(reqBody)

		firstReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		firstReq.Header.Set("Content-Type", "application/json")
		firstReq.Header.Set("X-Tenant-ID", "tenant-001")
		firstResp := httptest.NewRecorder()
		server.Router().ServeHTTP(firstResp, firstReq)
		if firstResp.Code != http.StatusOK {
			t.Fatalf("expected first evaluation 200, got %d: %s", firstResp.Code, firstResp.Body.String())
		}

		secondReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		secondReq.Header.Set("Content-Type", "application/json")
		secondReq.Header.Set("X-Tenant-ID", "tenant-001")
		secondResp := httptest.NewRecorder()
		server.Router().ServeHTTP(secondResp, secondReq)
		if secondResp.Code != http.StatusConflict {
			t.Fatalf("expected duplicate transaction id 409, got %d: %s", secondResp.Code, secondResp.Body.String())
		}

		otherTenantReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
		otherTenantReq.Header.Set("Content-Type", "application/json")
		otherTenantReq.Header.Set("X-Tenant-ID", "tenant-002")
		otherTenantResp := httptest.NewRecorder()
		server.Router().ServeHTTP(otherTenantResp, otherTenantReq)
		if otherTenantResp.Code != http.StatusOK {
			t.Fatalf("expected same client transaction id in another tenant to succeed, got %d: %s", otherTenantResp.Code, otherTenantResp.Body.String())
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

	t.Run("TransactionPersistenceFailure", func(t *testing.T) {
		repo := &failingRepository{saveTransactionErr: errors.New("write failed")}
		server := createTestServerWithRepository(repo)

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

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("EvaluationPersistenceFailure", func(t *testing.T) {
		repo := &failingRepository{saveEvaluationErr: errors.New("write failed")}
		server := createTestServerWithRepository(repo)

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

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("ReadEndpointsReturnNotFoundOnlyForMissingRecords", func(t *testing.T) {
		repo := &failingRepository{
			getTransactionErr: repository.ErrNotFound,
			getEvaluationErr:  repository.ErrNotFound,
		}
		server := createTestServerWithRepository(repo)

		txReq := httptest.NewRequest(http.MethodGet, "/transactions/missing-tx", nil)
		txReq.Header.Set("X-Tenant-ID", "tenant-001")
		txResp := httptest.NewRecorder()
		server.Router().ServeHTTP(txResp, txReq)
		if txResp.Code != http.StatusNotFound {
			t.Fatalf("expected missing transaction to return 404, got %d: %s", txResp.Code, txResp.Body.String())
		}

		evalReq := httptest.NewRequest(http.MethodGet, "/evaluations/missing-eval", nil)
		evalReq.Header.Set("X-Tenant-ID", "tenant-001")
		evalResp := httptest.NewRecorder()
		server.Router().ServeHTTP(evalResp, evalReq)
		if evalResp.Code != http.StatusNotFound {
			t.Fatalf("expected missing evaluation to return 404, got %d: %s", evalResp.Code, evalResp.Body.String())
		}
	})

	t.Run("ReadEndpointsReturnInternalServerErrorForRepositoryFailures", func(t *testing.T) {
		repo := &failingRepository{
			getTransactionErr: errors.New("decode transaction failed"),
			getEvaluationErr:  errors.New("decode evaluation failed"),
		}
		server := createTestServerWithRepository(repo)

		txReq := httptest.NewRequest(http.MethodGet, "/transactions/bad-tx", nil)
		txReq.Header.Set("X-Tenant-ID", "tenant-001")
		txResp := httptest.NewRecorder()
		server.Router().ServeHTTP(txResp, txReq)
		if txResp.Code != http.StatusInternalServerError {
			t.Fatalf("expected transaction read failure to return 500, got %d: %s", txResp.Code, txResp.Body.String())
		}

		evalReq := httptest.NewRequest(http.MethodGet, "/evaluations/bad-eval", nil)
		evalReq.Header.Set("X-Tenant-ID", "tenant-001")
		evalResp := httptest.NewRecorder()
		server.Router().ServeHTTP(evalResp, evalReq)
		if evalResp.Code != http.StatusInternalServerError {
			t.Fatalf("expected evaluation read failure to return 500, got %d: %s", evalResp.Code, evalResp.Body.String())
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

	t.Run("CreateRuleLoadsEngineImmediately", func(t *testing.T) {
		server, cleanup := createPersistentTestServer(t)
		defer cleanup()

		rulePayload := map[string]any{
			"id":          "immediate-rule",
			"name":        "Immediate Rule",
			"description": "Should be active immediately after create",
			"expression":  "1 == 1",
			"bands": []map[string]any{
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
		setAdminAuth(createReq)

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
			Count  int    `json:"count"`
			Source string `json:"source"`
			Rules  []struct {
				ID string `json:"id"`
			} `json:"rules"`
		}
		if err := json.Unmarshal(rulesResp.Body.Bytes(), &listed); err != nil {
			t.Fatalf("failed to parse rules list: %v", err)
		}

		if listed.Count != 1 {
			t.Fatalf("expected created rule to be loaded immediately, got %d loaded rules", listed.Count)
		}
		if listed.Source != "active-engine" {
			t.Fatalf("expected rules source active-engine, got %q", listed.Source)
		}

		if listed.Rules[0].ID != "immediate-rule" {
			t.Fatalf("expected immediate-rule to be loaded, got %s", listed.Rules[0].ID)
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
		if evalResult.Status != domain.StatusAlert {
			t.Fatalf("expected ALRT after immediate rule load, got %s", evalResult.Status)
		}
	})

	t.Run("AdminTokenProtectsRuleMutationOnly", func(t *testing.T) {
		engine, _ := rules.NewEngine(nil, 5)
		server, cleanup := createPersistentTestServerWithEngineAndAdminToken(t, engine, "secret-token")
		defer cleanup()

		rulePayload := map[string]any{
			"id":         "protected-rule",
			"name":       "Protected Rule",
			"expression": "1 == 1",
			"weight":     1.0,
			"enabled":    true,
		}
		body, _ := json.Marshal(rulePayload)

		createReq := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(body))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("X-Tenant-ID", "tenant-001")
		createResp := httptest.NewRecorder()
		server.Router().ServeHTTP(createResp, createReq)
		if createResp.Code != http.StatusUnauthorized {
			t.Fatalf("expected rule mutation without admin token to return 401, got %d", createResp.Code)
		}
		if !strings.Contains(createResp.Body.String(), "admin token is invalid or missing") {
			t.Fatalf("expected invalid-or-missing admin token error, got %s", createResp.Body.String())
		}

		wrongTokenReq := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(body))
		wrongTokenReq.Header.Set("Content-Type", "application/json")
		wrongTokenReq.Header.Set("X-Tenant-ID", "tenant-001")
		wrongTokenReq.Header.Set("Authorization", "Bearer wrong-token")
		wrongTokenResp := httptest.NewRecorder()
		server.Router().ServeHTTP(wrongTokenResp, wrongTokenReq)
		if wrongTokenResp.Code != http.StatusUnauthorized {
			t.Fatalf("expected wrong admin token to return 401, got %d", wrongTokenResp.Code)
		}
		if !strings.Contains(wrongTokenResp.Body.String(), "admin token is invalid or missing") {
			t.Fatalf("expected invalid-or-missing admin token error, got %s", wrongTokenResp.Body.String())
		}

		evalReqBody := TransactionRequest{
			Type:     "transfer",
			Debtor:   PartyInfo{ID: "d1", AccountID: "a1"},
			Creditor: PartyInfo{ID: "c1", AccountID: "a2"},
			Amount:   AmountInfo{Value: 100, Currency: "USD"},
		}
		evalBody, _ := json.Marshal(evalReqBody)
		evalReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(evalBody))
		evalReq.Header.Set("Content-Type", "application/json")
		evalReq.Header.Set("X-Tenant-ID", "tenant-001")
		evalResp := httptest.NewRecorder()
		server.Router().ServeHTTP(evalResp, evalReq)
		if evalResp.Code != http.StatusOK {
			t.Fatalf("expected evaluation without admin token to remain available, got %d", evalResp.Code)
		}

		authorizedReq := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(body))
		authorizedReq.Header.Set("Content-Type", "application/json")
		authorizedReq.Header.Set("X-Tenant-ID", "tenant-001")
		authorizedReq.Header.Set("Authorization", "Bearer secret-token")
		authorizedResp := httptest.NewRecorder()
		server.Router().ServeHTTP(authorizedResp, authorizedReq)
		if authorizedResp.Code != http.StatusCreated {
			t.Fatalf("expected rule mutation with admin token to return 201, got %d: %s", authorizedResp.Code, authorizedResp.Body.String())
		}

		headerReq := httptest.NewRequest(http.MethodPost, "/rules/reload", nil)
		headerReq.Header.Set("X-Tenant-ID", "tenant-001")
		headerReq.Header.Set("X-Osprey-Admin-Token", "secret-token")
		headerResp := httptest.NewRecorder()
		server.Router().ServeHTTP(headerResp, headerReq)
		if headerResp.Code != http.StatusOK {
			t.Fatalf("expected rule reload with admin token header to return 200, got %d: %s", headerResp.Code, headerResp.Body.String())
		}
	})

	t.Run("UnconfiguredAdminTokenRejectsMutations", func(t *testing.T) {
		cfg := domain.ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30,
			WriteTimeout: 30,
		}
		engine, _ := rules.NewEngine(nil, 5)
		processor := tadp.NewProcessor()
		server := NewServer(cfg, nil, nil, nil, engine, rules.NewTypologyEngine(), processor, "test-v1", domain.ModeDetection)

		rulePayload := map[string]any{
			"id":         "unconfigured-admin-rule",
			"name":       "Unconfigured Admin Rule",
			"expression": "1 == 1",
			"weight":     1.0,
			"enabled":    true,
		}
		body, _ := json.Marshal(rulePayload)
		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected unconfigured admin token to return 503, got %d: %s", resp.Code, resp.Body.String())
		}
		if !strings.Contains(resp.Body.String(), "admin token is not configured") {
			t.Fatalf("expected unconfigured admin token error, got %s", resp.Body.String())
		}
	})

	t.Run("RuleMutationRequiresRepository", func(t *testing.T) {
		server := createTestServerWithRepository(nil)

		rulePayload := map[string]any{
			"id":         "repo-required-rule",
			"name":       "Repository Required Rule",
			"expression": "1 == 1",
			"weight":     1.0,
			"enabled":    true,
		}
		body, _ := json.Marshal(rulePayload)
		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected missing repository to return 503, got %d: %s", resp.Code, resp.Body.String())
		}
	})
}

func TestTypologyMutationValidation(t *testing.T) {
	engine, _ := rules.NewEngine(nil, 5)
	if err := engine.LoadRule(&domain.RuleConfig{
		ID:         "loaded-rule",
		Name:       "Loaded Rule",
		Expression: "1 == 1",
		Weight:     1.0,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("failed to load test rule: %v", err)
	}
	server, cleanup := createPersistentTestServerWithEngine(t, engine)
	defer cleanup()

	createPayload := map[string]any{
		"id":             "typology-001",
		"name":           "Test Typology",
		"alertThreshold": 0.5,
		"enabled":        true,
		"rules": []map[string]any{
			{"ruleId": "loaded-rule", "weight": 1.0},
		},
	}
	createBody, _ := json.Marshal(createPayload)
	createReq := httptest.NewRequest(http.MethodPost, "/typologies", bytes.NewBuffer(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Tenant-ID", "tenant-001")
	setAdminAuth(createReq)
	createResp := httptest.NewRecorder()
	server.Router().ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected typology create 201, got %d: %s", createResp.Code, createResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/typologies", nil)
	listReq.Header.Set("X-Tenant-ID", "tenant-001")
	listResp := httptest.NewRecorder()
	server.Router().ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected typology list 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Count  int    `json:"count"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &listed); err != nil {
		t.Fatalf("failed to decode typology list: %v", err)
	}
	if listed.Count != 1 {
		t.Fatalf("expected one active typology, got %d", listed.Count)
	}
	if listed.Source != "active-engine" {
		t.Fatalf("expected typology source active-engine, got %q", listed.Source)
	}

	t.Run("UpdateRejectsMissingName", func(t *testing.T) {
		payload := map[string]any{
			"alertThreshold": 0.5,
			"enabled":        true,
			"rules": []map[string]any{
				{"ruleId": "loaded-rule", "weight": 1.0},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/typologies/typology-001", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected missing name to return 400, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("UpdateRejectsUnknownRule", func(t *testing.T) {
		payload := map[string]any{
			"name":           "Test Typology",
			"alertThreshold": 0.5,
			"enabled":        true,
			"rules": []map[string]any{
				{"ruleId": "missing-rule", "weight": 1.0},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/typologies/typology-001", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected unknown rule to return 400, got %d: %s", resp.Code, resp.Body.String())
		}
		if !strings.Contains(resp.Body.String(), "missing-rule") {
			t.Fatalf("expected response to identify missing rule, got %s", resp.Body.String())
		}
	})

	t.Run("UpdateRejectsInvalidThreshold", func(t *testing.T) {
		payload := map[string]any{
			"name":           "Test Typology",
			"alertThreshold": 0,
			"enabled":        true,
			"rules": []map[string]any{
				{"ruleId": "loaded-rule", "weight": 1.0},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/typologies/typology-001", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected invalid threshold to return 400, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("TypologyMutationRequiresRepository", func(t *testing.T) {
		noRepoServer := createTestServerWithRepository(nil)

		payload := map[string]any{
			"id":             "typology-no-repo",
			"name":           "No Repo Typology",
			"alertThreshold": 0.5,
			"enabled":        true,
			"rules": []map[string]any{
				{"ruleId": "loaded-rule", "weight": 1.0},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/typologies", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		noRepoServer.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected missing repository to return 503, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("DeleteTypologyMapsNotFoundOnlyTo404", func(t *testing.T) {
		notFoundServer := createTestServerWithRepository(&failingRepository{deleteTypologyErr: repository.ErrNotFound})
		notFoundReq := httptest.NewRequest(http.MethodDelete, "/typologies/missing-typology", nil)
		notFoundReq.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(notFoundReq)
		notFoundResp := httptest.NewRecorder()
		notFoundServer.Router().ServeHTTP(notFoundResp, notFoundReq)
		if notFoundResp.Code != http.StatusNotFound {
			t.Fatalf("expected not-found delete error to return 404, got %d: %s", notFoundResp.Code, notFoundResp.Body.String())
		}

		dbErrServer := createTestServerWithRepository(&failingRepository{deleteTypologyErr: errors.New("database unavailable")})
		dbErrReq := httptest.NewRequest(http.MethodDelete, "/typologies/broken-typology", nil)
		dbErrReq.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(dbErrReq)
		dbErrResp := httptest.NewRecorder()
		dbErrServer.Router().ServeHTTP(dbErrResp, dbErrReq)
		if dbErrResp.Code != http.StatusInternalServerError {
			t.Fatalf("expected operational delete error to return 500, got %d: %s", dbErrResp.Code, dbErrResp.Body.String())
		}
	})
}

func TestRuleMutationValidation(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	t.Run("RejectsInvalidWeight", func(t *testing.T) {
		payload := map[string]any{
			"id":         "invalid-weight",
			"name":       "Invalid Weight",
			"expression": "1 == 1",
			"weight":     1.5,
			"enabled":    true,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected invalid weight to return 400, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("RejectsInvalidBandRange", func(t *testing.T) {
		payload := map[string]any{
			"id":         "invalid-band",
			"name":       "Invalid Band",
			"expression": "1 == 1",
			"weight":     1.0,
			"enabled":    true,
			"bands": []map[string]any{
				{"lowerLimit": 1.0, "upperLimit": 0.5, "subRuleRef": ".fail", "reason": "Invalid range"},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected invalid band range to return 400, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("TrimsRuleAndBandFields", func(t *testing.T) {
		payload := map[string]any{
			"id":         " trimmed-rule ",
			"name":       " Trimmed Rule ",
			"expression": " 1 == 1 ",
			"weight":     1.0,
			"enabled":    true,
			"bands": []map[string]any{
				{"lowerLimit": 1.0, "subRuleRef": " .fail ", "reason": " Always fail "},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-001")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusCreated {
			t.Fatalf("expected trimmed rule to create, got %d: %s", resp.Code, resp.Body.String())
		}

		var created struct {
			Rule domain.RuleConfig `json:"rule"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
			t.Fatalf("failed to decode created rule: %v", err)
		}
		if created.Rule.ID != "trimmed-rule" || created.Rule.Name != "Trimmed Rule" || created.Rule.Expression != "1 == 1" {
			t.Fatalf("expected trimmed rule fields, got %#v", created.Rule)
		}
		if len(created.Rule.Bands) != 1 || created.Rule.Bands[0].SubRuleRef != ".fail" || created.Rule.Bands[0].Reason != "Always fail" {
			t.Fatalf("expected trimmed band fields, got %#v", created.Rule.Bands)
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

		var resp map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)

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

		var resp map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode ready response: %v", err)
		}
		if resp["ready"] != "true" {
			t.Fatalf("expected ready=true, got %q", resp["ready"])
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

		var resp map[string]any
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

	t.Run("TenantMiddlewareTrimsAndRejectsBlankID", func(t *testing.T) {
		var capturedTenantID string

		handler := TenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedTenantID = GetTenantID(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Tenant-ID", "  tenant-trimmed  ")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected trimmed tenant to be accepted, got %d", rr.Code)
		}
		if capturedTenantID != "tenant-trimmed" {
			t.Fatalf("expected trimmed tenant ID, got %q", capturedTenantID)
		}

		blankReq := httptest.NewRequest(http.MethodGet, "/", nil)
		blankReq.Header.Set("X-Tenant-ID", "   ")
		blankResp := httptest.NewRecorder()
		handler.ServeHTTP(blankResp, blankReq)

		if blankResp.Code != http.StatusBadRequest {
			t.Fatalf("expected blank tenant to be rejected, got %d", blankResp.Code)
		}
		if blankResp.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("expected JSON error response, got %q", blankResp.Header().Get("Content-Type"))
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
		if rr.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("expected JSON error response, got %q", rr.Header().Get("Content-Type"))
		}
	})

	t.Run("CORSMiddlewareUsesHeaderAuthWithoutCredentials", func(t *testing.T) {
		handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodOptions, "/rules", nil)
		req.Header.Set("Origin", "https://client.example")
		req.Header.Set("Access-Control-Request-Headers", "authorization,x-tenant-id,x-osprey-admin-token")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected preflight status 204, got %d", rr.Code)
		}
		if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("expected wildcard CORS origin, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
		}
		if rr.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("expected credentials to be disabled for header-token auth")
		}
		if rr.Header().Get("Access-Control-Allow-Headers") == "" {
			t.Fatalf("expected allowed CORS headers")
		}
	})
}
