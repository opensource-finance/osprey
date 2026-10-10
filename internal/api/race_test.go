package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// slowSaveRepo wraps a real Repository and blocks the first SaveTransaction
// call until the release channel is closed. It signals reached (closed once)
// the first time SaveTransaction is called, so a test can deterministically
// reproduce the TOCTOU window: the evaluate handler has already passed the
// compliance-mode entry guard (typologies loaded) and is now blocked inside
// SaveTransaction, while a concurrent admin request empties the typology set.
//
// evalsSaved counts SaveEvaluation calls so the test can prove a racy
// compliance-mode request does not persist a detection-mode decision.
type slowSaveRepo struct {
	domain.Repository
	reached chan struct{}
	release chan struct{}
	once    sync.Once

	mu         sync.Mutex
	evalsSaved int
}

func (r *slowSaveRepo) SaveTransaction(ctx context.Context, tenantID string, tx *domain.Transaction) error {
	r.once.Do(func() { close(r.reached) })
	<-r.release
	return r.Repository.SaveTransaction(ctx, tenantID, tx)
}

func (r *slowSaveRepo) SaveEvaluation(ctx context.Context, tenantID string, eval *domain.Evaluation) error {
	r.mu.Lock()
	r.evalsSaved++
	r.mu.Unlock()
	return r.Repository.SaveEvaluation(ctx, tenantID, eval)
}

func (r *slowSaveRepo) evalsSavedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.evalsSaved
}

// TestComplianceTOCTOURaceRejectsOnEmptyTypologyMidRequest deterministically
// reproduces the TOCTOU exploit chain through the real HTTP handler and
// asserts the fix: a compliance-mode evaluation whose typology set is emptied
// mid-request (concurrent DELETE on the last enabled typology) is rejected
// with 503, NOT answered as a detection-mode decision with HTTP 200.
//
// Pre-fix behavior (the bug): the evaluate response was HTTP 200 with status
// ALRT (detection fallback), and a "detection-summary" typology was persisted.
// Post-fix: 503, no evaluation persisted, and the same input re-evaluated with
// typologies loaded correctly yields NALT.
func TestComplianceTOCTOURaceRejectsOnEmptyTypologyMidRequest(t *testing.T) {
	ctx := context.Background()

	dbFile, err := os.CreateTemp("", "osprey-race-*.db")
	if err != nil {
		t.Fatalf("failed to create temp database: %v", err)
	}
	dbPath := dbFile.Name()
	if err := dbFile.Close(); err != nil {
		t.Fatalf("failed to close temp database: %v", err)
	}
	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: dbPath})
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
	defer cleanup()

	typo := &domain.Typology{
		ID:             "structuring-check",
		TenantID:       domain.GlobalTenantID,
		Name:           "Structuring Detection",
		Version:        "1.0.0",
		AlertThreshold: 0.9,
		Enabled:        true,
		Rules:          []domain.TypologyRuleWeight{{RuleID: "high-amount-rule", Weight: 1.0}},
	}
	if err := repo.SaveTypology(ctx, domain.GlobalTenantID, typo); err != nil {
		t.Fatalf("failed to seed typology: %v", err)
	}

	// Rule: amount > 100 -> score 0.8. With typology loaded:
	//   typology score = 0.8 * 1.0 = 0.8 < 0.9 -> not triggered -> NALT.
	// Detection aggregate = 0.8 >= 0.7 -> ALRT (the divergent detection result).
	engine, _ := rules.NewEngine(nil, 5)
	if err := engine.LoadRule(&domain.RuleConfig{
		ID:         "high-amount-rule",
		Name:       "High Amount Rule",
		Version:    "1.0.0",
		Expression: "amount > 100.0 ? 0.8 : 0.0",
		Weight:     1.0,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("failed to load rule: %v", err)
	}

	typologyEngine := rules.NewTypologyEngine()
	typologyEngine.LoadTypologies([]*domain.Typology{typo})

	processor := tadp.NewComplianceProcessor() // AlertThreshold 0.7

	reached := make(chan struct{})
	release := make(chan struct{})
	slowRepo := &slowSaveRepo{Repository: repo, reached: reached, release: release}

	cfg := domain.ServerConfig{
		Host:         "localhost",
		Port:         8080,
		ReadTimeout:  30,
		WriteTimeout: 30,
		AdminToken:   testAdminToken,
	}
	server := NewServer(cfg, slowRepo, nil, nil, engine, typologyEngine, processor, "test-v1", domain.ModeCompliance)
	router := server.Router()

	// Fire POST /evaluate in a goroutine. The handler passes the entry guard
	// (typologies loaded), then blocks inside SaveTransaction.
	evalBody, _ := json.Marshal(TransactionRequest{
		Type:     "transfer",
		Debtor:   PartyInfo{ID: "debtor-race", AccountID: "a1"},
		Creditor: PartyInfo{ID: "creditor-race", AccountID: "a2"},
		Amount:   AmountInfo{Value: 200.0, Currency: "USD"}, // amount>100 -> score 0.8
	})
	evalReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(evalBody))
	evalReq.Header.Set("Content-Type", "application/json")
	evalReq.Header.Set("X-Tenant-ID", "tenant-race")
	evalResp := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Go(func() {
		router.ServeHTTP(evalResp, evalReq)
	})

	<-reached // handler is past the entry guard, blocked at SaveTransaction

	// Concurrent admin operation: delete the last typology. This runs the real
	// DELETE handler: repo.DeleteTypology + reloadTypologiesFromRepository, the
	// latter calling typologyEngine.ReloadTypologies([]) which bumps the
	// generation and empties the set.
	delReq := httptest.NewRequest(http.MethodDelete, "/typologies/structuring-check", nil)
	delReq.Header.Set("X-Tenant-ID", "tenant-race")
	setAdminAuth(delReq)
	delResp := httptest.NewRecorder()
	router.ServeHTTP(delResp, delReq)
	if delResp.Code != http.StatusOK {
		t.Fatalf("delete typology: expected 200, got %d: %s", delResp.Code, delResp.Body.String())
	}
	if typologyEngine.TypologyCount() != 0 {
		t.Fatalf("expected typology engine empty after delete, got count=%d", typologyEngine.TypologyCount())
	}

	// Release the evaluate handler. It completes rule evaluation, then the
	// atomic generation check detects the typology set changed and rejects.
	close(release)
	wg.Wait()

	if evalResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("after race: expected 503 (reject), got %d: %s", evalResp.Code, evalResp.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(evalResp.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if !strings.Contains(errResp["error"], "typology configuration changed") {
		t.Fatalf("expected 'typology configuration changed' error, got %q", errResp["error"])
	}

	// The racy request must not have persisted any evaluation (no silent
	// detection-mode decision in the audit trail).
	if got := slowRepo.evalsSavedCount(); got != 0 {
		t.Fatalf("racy compliance request must not persist an evaluation; got %d saved", got)
	}

	// Regression: the same input re-evaluated with typologies loaded must yield
	// NALT (correct compliance behavior), proving no regression in the happy
	// path and that the divergence was the race, not the rule/typology setup.
	typologyEngine.LoadTypologies([]*domain.Typology{typo})
	if typologyEngine.TypologyCount() != 1 {
		t.Fatalf("re-load: expected count 1, got %d", typologyEngine.TypologyCount())
	}
	reBody, _ := json.Marshal(TransactionRequest{
		ID:       "tx-retry", // distinct tx id; the racy tx was already saved
		Type:     "transfer",
		Debtor:   PartyInfo{ID: "debtor-race", AccountID: "a1"},
		Creditor: PartyInfo{ID: "creditor-race", AccountID: "a2"},
		Amount:   AmountInfo{Value: 200.0, Currency: "USD"},
	})
	reReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(reBody))
	reReq.Header.Set("Content-Type", "application/json")
	reReq.Header.Set("X-Tenant-ID", "tenant-race")
	reResp := httptest.NewRecorder()
	router.ServeHTTP(reResp, reReq)
	if reResp.Code != http.StatusOK {
		t.Fatalf("re-evaluate: expected 200, got %d: %s", reResp.Code, reResp.Body.String())
	}
	var reEval EvaluateResponse
	if err := json.Unmarshal(reResp.Body.Bytes(), &reEval); err != nil {
		t.Fatalf("failed to parse re-evaluate response: %v", err)
	}
	if reEval.Status != domain.StatusNoAlert {
		t.Fatalf("re-evaluate with typologies loaded: expected NALT (typology score 0.8 < 0.9), got %s", reEval.Status)
	}
	if got := slowRepo.evalsSavedCount(); got != 1 {
		t.Fatalf("re-evaluate: expected exactly 1 evaluation persisted, got %d", got)
	}
}

// TestComplianceTOCTOURaceRejectsOnDisableMidRequest covers the other typology
// mutation path that empties the set: PUT /typologies/{id} disabling the last
// enabled typology (only Enabled typologies are loaded).
func TestComplianceTOCTOURaceRejectsOnDisableMidRequest(t *testing.T) {
	ctx := context.Background()

	dbFile, err := os.CreateTemp("", "osprey-race-disable-*.db")
	if err != nil {
		t.Fatalf("failed to create temp database: %v", err)
	}
	dbPath := dbFile.Name()
	if err := dbFile.Close(); err != nil {
		t.Fatalf("failed to close temp database: %v", err)
	}
	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: dbPath})
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
	defer cleanup()

	typo := &domain.Typology{
		ID:             "structuring-check",
		TenantID:       domain.GlobalTenantID,
		Name:           "Structuring Detection",
		Version:        "1.0.0",
		AlertThreshold: 0.9,
		Enabled:        true,
		Rules:          []domain.TypologyRuleWeight{{RuleID: "high-amount-rule", Weight: 1.0}},
	}
	if err := repo.SaveTypology(ctx, domain.GlobalTenantID, typo); err != nil {
		t.Fatalf("failed to seed typology: %v", err)
	}

	engine, _ := rules.NewEngine(nil, 5)
	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "high-amount-rule", Name: "High Amount Rule", Version: "1.0.0",
		Expression: "amount > 100.0 ? 0.8 : 0.0", Weight: 1.0, Enabled: true,
	}); err != nil {
		t.Fatalf("failed to load rule: %v", err)
	}

	typologyEngine := rules.NewTypologyEngine()
	typologyEngine.LoadTypologies([]*domain.Typology{typo})

	processor := tadp.NewComplianceProcessor()

	reached := make(chan struct{})
	release := make(chan struct{})
	slowRepo := &slowSaveRepo{Repository: repo, reached: reached, release: release}
	cfg := domain.ServerConfig{Host: "localhost", Port: 8080, ReadTimeout: 30, WriteTimeout: 30, AdminToken: testAdminToken}
	server := NewServer(cfg, slowRepo, nil, nil, engine, typologyEngine, processor, "test-v1", domain.ModeCompliance)
	router := server.Router()

	evalBody, _ := json.Marshal(TransactionRequest{
		Type: "transfer", Debtor: PartyInfo{ID: "d", AccountID: "a"}, Creditor: PartyInfo{ID: "c", AccountID: "b"},
		Amount: AmountInfo{Value: 200.0, Currency: "USD"},
	})
	evalReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(evalBody))
	evalReq.Header.Set("Content-Type", "application/json")
	evalReq.Header.Set("X-Tenant-ID", "tenant-race")
	evalResp := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Go(func() {
		router.ServeHTTP(evalResp, evalReq)
	})
	<-reached

	// Disable the last enabled typology via PUT. ReloadTypologies only loads
	// Enabled typologies, disabling the last one empties the set.
	disableBody, _ := json.Marshal(map[string]any{
		"name":           "Structuring Detection",
		"alertThreshold": 0.9,
		"enabled":        false,
		"rules":          []map[string]any{{"ruleId": "high-amount-rule", "weight": 1.0}},
	})
	disReq := httptest.NewRequest(http.MethodPut, "/typologies/structuring-check", bytes.NewBuffer(disableBody))
	disReq.Header.Set("Content-Type", "application/json")
	disReq.Header.Set("X-Tenant-ID", "tenant-race")
	setAdminAuth(disReq)
	disResp := httptest.NewRecorder()
	router.ServeHTTP(disResp, disReq)
	if disResp.Code != http.StatusOK {
		t.Fatalf("disable typology: expected 200, got %d: %s", disResp.Code, disResp.Body.String())
	}
	if typologyEngine.TypologyCount() != 0 {
		t.Fatalf("expected typology engine empty after disable, got count=%d", typologyEngine.TypologyCount())
	}

	close(release)
	wg.Wait()

	if evalResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("after race (disable path): expected 503, got %d: %s", evalResp.Code, evalResp.Body.String())
	}
	if got := slowRepo.evalsSavedCount(); got != 0 {
		t.Fatalf("racy compliance request must not persist an evaluation; got %d saved", got)
	}
}
