package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// newZeroRulesServer builds a detection-mode server on a real SQLite repo
// with no rules loaded in the engine. It returns the server, the engine
// (for RulesCount assertions), the repo (for assertions on stored
// evaluations), and a cleanup function.
func newZeroRulesServer(t *testing.T) (*Server, *rules.Engine, domain.Repository, func()) {
	t.Helper()
	dbFile, err := os.CreateTemp("", "osprey-zero-rules-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	dbPath := dbFile.Name()
	if err := dbFile.Close(); err != nil {
		t.Fatalf("close temp db: %v", err)
	}
	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: dbPath})
	if err != nil {
		_ = os.Remove(dbPath)
		t.Fatalf("create repository: %v", err)
	}
	cleanup := func() {
		_ = repo.Close()
		_ = os.Remove(dbPath)
		_ = os.Remove(dbPath + "-shm")
		_ = os.Remove(dbPath + "-wal")
	}
	engine, _ := rules.NewEngine(nil, 5)
	cfg := domain.ServerConfig{
		Host:         "localhost",
		Port:         8080,
		ReadTimeout:  30,
		WriteTimeout: 30,
		AdminToken:   testAdminToken,
	}
	server := NewServer(cfg, repo, nil, nil, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), "test-v1", domain.ModeDetection)
	return server, engine, repo, cleanup
}

func zeroRulesEvalBody() []byte {
	body, _ := json.Marshal(TransactionRequest{
		Type:     "transfer",
		Debtor:   PartyInfo{ID: "debtor-zero", AccountID: "a1"},
		Creditor: PartyInfo{ID: "creditor-zero", AccountID: "a2"},
		Amount:   AmountInfo{Value: 500.0, Currency: "USD"},
	})
	return body
}

// TestEvaluateWithZeroRulesFailsClosed covers the zero-rules fail-open bug at
// the API boundary. A detection-mode server with zero rules loaded used to
// return NALT with score 0 for every transaction. Now POST /evaluate fails
// closed to ALRT and the stored evaluation carries the no-rules marker.
func TestEvaluateWithZeroRulesFailsClosed(t *testing.T) {
	ctx := context.Background()
	server, engine, repo, cleanup := newZeroRulesServer(t)
	defer cleanup()
	router := server.Router()

	if engine.RulesCount() != 0 {
		t.Fatalf("precondition: expected 0 rules, got %d", engine.RulesCount())
	}

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(zeroRulesEvalBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-zero")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("evaluate: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp EvaluateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Status != domain.StatusAlert {
		t.Fatalf("expected ALRT (fail closed) for zero rules, got %s with score %.2f", resp.Status, resp.Score)
	}
	if resp.Score != 0 {
		t.Errorf("expected score 0 for fail-closed ALRT, got %.2f", resp.Score)
	}

	// The stored evaluation must carry the no-rules marker so the audit
	// record shows why it alerted, not a blank NALT a reviewer could miss.
	eval, err := repo.GetEvaluation(ctx, "tenant-zero", resp.EvaluationID)
	if err != nil {
		t.Fatalf("GetEvaluation: %v", err)
	}
	if len(eval.TypologyResults) != 1 || eval.TypologyResults[0].TypologyID != tadp.NoRulesID {
		t.Fatalf("expected stored typology result %q, got %+v", tadp.NoRulesID, eval.TypologyResults)
	}
	if !eval.TypologyResults[0].Triggered {
		t.Error("expected the no-rules marker result to be triggered")
	}
	if eval.Metadata.RulesEvaluated != 0 {
		t.Errorf("expected stored RulesEvaluated 0, got %d", eval.Metadata.RulesEvaluated)
	}
}

// TestEvaluateAfterLastRuleDeletedFailsClosed exercises the runtime blinding
// path the bug report describes. An operator with rule-management API
// credentials deletes the only rule. The engine reloads to zero rules and
// the next evaluation fails closed to ALRT, not a silent NALT.
func TestEvaluateAfterLastRuleDeletedFailsClosed(t *testing.T) {
	ctx := context.Background()
	server, engine, repo, cleanup := newZeroRulesServer(t)
	defer cleanup()
	router := server.Router()

	// Seed one rule through the API so the engine has one loaded rule that
	// only flags very high amounts. A normal transaction passes (NALT).
	createBody, _ := json.Marshal(map[string]any{
		"id": "only-rule", "name": "Only Rule",
		"expression": "amount > 100000.0 ? 1.0 : 0.0",
		"weight":     1.0, "enabled": true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Tenant-ID", "tenant-del")
	setAdminAuth(createReq)
	createRR := httptest.NewRecorder()
	router.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create rule: expected 201, got %d: %s", createRR.Code, createRR.Body.String())
	}
	if engine.RulesCount() != 1 {
		t.Fatalf("precondition: expected 1 rule loaded, got %d", engine.RulesCount())
	}

	// Baseline: a normal transaction passes while the rule is loaded.
	baseReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(zeroRulesEvalBody()))
	baseReq.Header.Set("Content-Type", "application/json")
	baseReq.Header.Set("X-Tenant-ID", "tenant-del")
	baseRR := httptest.NewRecorder()
	router.ServeHTTP(baseRR, baseReq)
	if baseRR.Code != http.StatusOK {
		t.Fatalf("baseline evaluate: expected 200, got %d: %s", baseRR.Code, baseRR.Body.String())
	}
	var baseResp EvaluateResponse
	if err := json.Unmarshal(baseRR.Body.Bytes(), &baseResp); err != nil {
		t.Fatalf("parse baseline response: %v", err)
	}
	if baseResp.Status != domain.StatusNoAlert {
		t.Fatalf("baseline with one rule: expected NALT, got %s", baseResp.Status)
	}

	// Delete the only rule. No typology references it, so the delete
	// succeeds and the engine reloads to zero rules.
	delReq := httptest.NewRequest(http.MethodDelete, "/rules/only-rule", nil)
	delReq.Header.Set("X-Tenant-ID", "tenant-del")
	setAdminAuth(delReq)
	delRR := httptest.NewRecorder()
	router.ServeHTTP(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("delete last rule: expected 200, got %d: %s", delRR.Code, delRR.Body.String())
	}
	if engine.RulesCount() != 0 {
		t.Fatalf("after delete: expected 0 rules loaded, got %d", engine.RulesCount())
	}

	// After the delete, the same transaction must fail closed to ALRT.
	afterReq := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(zeroRulesEvalBody()))
	afterReq.Header.Set("Content-Type", "application/json")
	afterReq.Header.Set("X-Tenant-ID", "tenant-del")
	afterRR := httptest.NewRecorder()
	router.ServeHTTP(afterRR, afterReq)
	if afterRR.Code != http.StatusOK {
		t.Fatalf("evaluate after delete: expected 200, got %d: %s", afterRR.Code, afterRR.Body.String())
	}
	var afterResp EvaluateResponse
	if err := json.Unmarshal(afterRR.Body.Bytes(), &afterResp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if afterResp.Status != domain.StatusAlert {
		t.Fatalf("expected ALRT (fail closed) after last rule deleted, got %s with score %.2f", afterResp.Status, afterResp.Score)
	}

	eval, err := repo.GetEvaluation(ctx, "tenant-del", afterResp.EvaluationID)
	if err != nil {
		t.Fatalf("GetEvaluation: %v", err)
	}
	if len(eval.TypologyResults) != 1 || eval.TypologyResults[0].TypologyID != tadp.NoRulesID {
		t.Fatalf("expected stored typology result %q, got %+v", tadp.NoRulesID, eval.TypologyResults)
	}
}

// TestEvaluateComplianceZeroRulesFailsClosed checks the compliance-mode half
// of the bug: with typologies loaded but zero rules, every typology scores 0
// and compliance mode used to return NALT. Now it fails closed to ALRT with
// the no-rules marker, which takes precedence over the compliance-no-
// typologies marker when both sets are empty.
func TestEvaluateComplianceZeroRulesFailsClosed(t *testing.T) {
	ctx := context.Background()
	dbFile, err := os.CreateTemp("", "osprey-compliance-zero-rules-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	dbPath := dbFile.Name()
	if err := dbFile.Close(); err != nil {
		t.Fatalf("close temp db: %v", err)
	}
	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: dbPath})
	if err != nil {
		_ = os.Remove(dbPath)
		t.Fatalf("create repository: %v", err)
	}
	defer func() {
		_ = repo.Close()
		_ = os.Remove(dbPath)
		_ = os.Remove(dbPath + "-shm")
		_ = os.Remove(dbPath + "-wal")
	}()

	// Seed one typology through the API. Compliance mode requires at least
	// one loaded typology to get past the entry 503, and the typology must
	// reference a rule. We create the typology directly on the typology
	// engine so we can keep the rule engine empty.
	typology := &domain.Typology{
		ID:             "compliance-zero-rules-typ",
		TenantID:       domain.GlobalTenantID,
		Name:           "Compliance Zero Rules",
		Version:        "1.0.0",
		AlertThreshold: 0.4,
		Enabled:        true,
		Rules:          []domain.TypologyRuleWeight{{RuleID: "absent-rule", Weight: 1.0}},
	}
	if err := repo.SaveTypology(ctx, domain.GlobalTenantID, typology); err != nil {
		t.Fatalf("save typology: %v", err)
	}

	engine, _ := rules.NewEngine(nil, 5) // zero rules
	typologyEngine := rules.NewTypologyEngine()
	typologyEngine.LoadTypologies([]*domain.Typology{typology})

	cfg := domain.ServerConfig{
		Host: "localhost", Port: 8080, ReadTimeout: 30, WriteTimeout: 30, AdminToken: testAdminToken,
	}
	server := NewServer(cfg, repo, nil, nil, engine, typologyEngine, tadp.NewComplianceProcessor(), "test-v1", domain.ModeCompliance)
	router := server.Router()

	if engine.RulesCount() != 0 {
		t.Fatalf("precondition: expected 0 rules, got %d", engine.RulesCount())
	}
	if typologyEngine.TypologyCount() != 1 {
		t.Fatalf("precondition: expected 1 typology, got %d", typologyEngine.TypologyCount())
	}

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(zeroRulesEvalBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-czr")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("evaluate: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp EvaluateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Status != domain.StatusAlert {
		t.Fatalf("expected ALRT (fail closed) for compliance with zero rules, got %s with score %.2f", resp.Status, resp.Score)
	}

	eval, err := repo.GetEvaluation(ctx, "tenant-czr", resp.EvaluationID)
	if err != nil {
		t.Fatalf("GetEvaluation: %v", err)
	}
	if len(eval.TypologyResults) != 1 || eval.TypologyResults[0].TypologyID != tadp.NoRulesID {
		t.Fatalf("expected no-rules marker to win over %q, got %+v", tadp.NoTypologiesID, eval.TypologyResults)
	}
	if eval.Metadata.RulesEvaluated != 0 {
		t.Errorf("expected stored RulesEvaluated 0, got %d", eval.Metadata.RulesEvaluated)
	}
}
