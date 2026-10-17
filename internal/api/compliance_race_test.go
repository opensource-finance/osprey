package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// blockingSaveRepo blocks the first SaveTransaction until release is closed,
// and closes reached when that call starts.
type blockingSaveRepo struct {
	domain.Repository
	reached chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *blockingSaveRepo) SaveTransaction(ctx context.Context, tenantID string, tx *domain.Transaction) error {
	r.once.Do(func() { close(r.reached) })
	<-r.release
	return r.Repository.SaveTransaction(ctx, tenantID, tx)
}

// TestComplianceTypologyDeletedMidRequestFailsClosed covers issue #54. The
// evaluate handler passes the "typologies loaded" check, then the last
// typology is deleted while the request is in SaveTransaction. Before the fix,
// the processor fell back to detection scoring. Here the rule score (0.5) is
// below the detection threshold (0.7) but above the typology threshold (0.4),
// so the fallback gave NALT for a transaction the typology flags.
func TestComplianceTypologyDeletedMidRequestFailsClosed(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: t.TempDir() + "/race.db"})
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	typology := &domain.Typology{
		ID:             "structuring-check",
		TenantID:       domain.GlobalTenantID,
		Name:           "Structuring Detection",
		Version:        "1.0.0",
		AlertThreshold: 0.4,
		Enabled:        true,
		Rules:          []domain.TypologyRuleWeight{{RuleID: "amount-rule", Weight: 1.0}},
	}
	if err := repo.SaveTypology(ctx, domain.GlobalTenantID, typology); err != nil {
		t.Fatalf("failed to seed typology: %v", err)
	}

	engine, _ := rules.NewEngine(nil, 5)
	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "amount-rule", Name: "Amount Rule", Version: "1.0.0",
		Expression: "amount > 100.0 ? 0.5 : 0.0", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("failed to load rule: %v", err)
	}
	typologyEngine := rules.NewTypologyEngine()
	typologyEngine.LoadTypologies([]*domain.Typology{typology})

	blocking := &blockingSaveRepo{Repository: repo, reached: make(chan struct{}), release: make(chan struct{})}
	cfg := domain.ServerConfig{Host: "localhost", Port: 8080, ReadTimeout: 30, WriteTimeout: 30, AdminToken: testAdminToken}
	server := NewServer(cfg, blocking, nil, nil, engine, typologyEngine, tadp.NewComplianceProcessor(), "test-v1", domain.ModeCompliance)
	router := server.Router()

	body, _ := json.Marshal(TransactionRequest{
		Type:     "transfer",
		Debtor:   PartyInfo{ID: "debtor-race", AccountID: "a1"},
		Creditor: PartyInfo{ID: "creditor-race", AccountID: "a2"},
		Amount:   AmountInfo{Value: 200.0, Currency: "USD"},
	})
	evalReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
	evalReq.Header.Set("Content-Type", "application/json")
	evalReq.Header.Set("X-Tenant-ID", "tenant-race")
	evalResp := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Go(func() { router.ServeHTTP(evalResp, evalReq) })
	<-blocking.reached

	delReq := httptest.NewRequest(http.MethodDelete, "/typologies/structuring-check", nil)
	delReq.Header.Set("X-Tenant-ID", "tenant-race")
	setAdminAuth(delReq)
	delResp := httptest.NewRecorder()
	router.ServeHTTP(delResp, delReq)
	if delResp.Code != http.StatusOK {
		t.Fatalf("delete typology: expected 200, got %d: %s", delResp.Code, delResp.Body.String())
	}
	if typologyEngine.TypologyCount() != 0 {
		t.Fatalf("precondition: expected 0 typologies after delete, got %d", typologyEngine.TypologyCount())
	}

	close(blocking.release)
	wg.Wait()

	if evalResp.Code != http.StatusOK {
		t.Fatalf("evaluate: expected 200, got %d: %s", evalResp.Code, evalResp.Body.String())
	}
	var resp EvaluateResponse
	if err := json.Unmarshal(evalResp.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != domain.StatusAlert {
		t.Errorf("expected ALRT (fail closed), got %s with score %.2f", resp.Status, resp.Score)
	}

	// The decision is stored, so the transaction is not orphaned, and the
	// audit record shows why it alerted.
	eval, err := repo.GetEvaluation(ctx, "tenant-race", resp.EvaluationID)
	if err != nil {
		t.Fatalf("GetEvaluation: %v", err)
	}
	if len(eval.TypologyResults) != 1 || eval.TypologyResults[0].TypologyID != tadp.NoTypologiesID {
		t.Errorf("expected stored typology result %q, got %+v", tadp.NoTypologiesID, eval.TypologyResults)
	}
}
