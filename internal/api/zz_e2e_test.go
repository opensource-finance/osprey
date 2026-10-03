package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// These tests cover the PUT /rules/{id} referential-integrity guard added to
// UpdateRule. Disabling (enabled:false) a rule that a loaded typology
// references must be refused with 409 (mirroring DeleteRule) unless the
// operator opts in with ?force=true, in which case the dependency is surfaced
// as a warnings array. The e2e tests reproduce the compliance-mode decision
// flip the unguarded path allowed and confirm it no longer happens silently.

const zzTenant = "t-zz"

// createComplianceServer builds a compliance-mode server backed by a real
// SQLite repository with fresh rule and typology engines, so rule disabling
// via the admin API can be exercised end-to-end against a loaded typology.
func createComplianceServer(t *testing.T) (*Server, func()) {
	t.Helper()
	dbFile, err := os.CreateTemp("", "osprey-api-*.db")
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
	engine, _ := rules.NewEngine(nil, 5)
	server := NewServer(
		domain.ServerConfig{Host: "localhost", Port: 8080, ReadTimeout: 30, WriteTimeout: 30, AdminToken: testAdminToken},
		repo, nil, nil, engine, rules.NewTypologyEngine(), tadp.NewComplianceProcessor(), "test-v1", domain.ModeCompliance,
	)
	return server, cleanup
}

func ruleCreateBody(id, expr string, weight float64, enabled bool) map[string]any {
	return map[string]any{"id": id, "name": id, "expression": expr, "weight": weight, "enabled": enabled}
}

func ruleCreateBodyBands(id, expr string, weight float64, enabled bool, bands []map[string]any) map[string]any {
	m := ruleCreateBody(id, expr, weight, enabled)
	m["bands"] = bands
	return m
}

func ruleUpdateBody(name, expr string, weight float64, enabled bool) map[string]any {
	return map[string]any{"name": name, "expression": expr, "weight": weight, "enabled": enabled}
}

func typologyBody(id, name string, threshold float64, ruleWeights []map[string]any) map[string]any {
	return map[string]any{
		"id":             id,
		"name":           name,
		"alertThreshold": threshold,
		"enabled":        true,
		"rules":          ruleWeights,
	}
}

func zzBandsReview() []map[string]any {
	return []map[string]any{
		{"lowerLimit": 1.0, "subRuleRef": ".review", "reason": "matched"},
		{"lowerLimit": 0.0, "upperLimit": 1.0, "subRuleRef": ".pass", "reason": "not matched"},
	}
}

func doCreateRule(t *testing.T, s *Server, tenant string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenant)
	setAdminAuth(req)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	return rr
}

func mustCreateRule(t *testing.T, s *Server, tenant string, body map[string]any) {
	t.Helper()
	rr := doCreateRule(t, s, tenant, body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create rule %v: expected 201, got %d: %s", body["id"], rr.Code, rr.Body.String())
	}
}

func mustCreateTypology(t *testing.T, s *Server, tenant string, body map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/typologies", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenant)
	setAdminAuth(req)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create typology %v: expected 201, got %d: %s", body["id"], rr.Code, rr.Body.String())
	}
}

func doUpdateRule(t *testing.T, s *Server, tenant, id string, body map[string]any, force bool) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	path := "/rules/" + id
	if force {
		path += "?force=true"
	}
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenant)
	setAdminAuth(req)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	return rr
}

func doDeleteRule(t *testing.T, s *Server, tenant, id string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/rules/"+id, nil)
	req.Header.Set("X-Tenant-ID", tenant)
	setAdminAuth(req)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	return rr
}

func doEvaluate(t *testing.T, s *Server, tenant string, amount float64) EvaluateResponse {
	t.Helper()
	body, _ := json.Marshal(TransactionRequest{
		Type:     "transfer",
		Debtor:   PartyInfo{ID: "debtor-zz", AccountID: "a1"},
		Creditor: PartyInfo{ID: "creditor-zz", AccountID: "a2"},
		Amount:   AmountInfo{Value: amount, Currency: "USD"},
	})
	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenant)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("evaluate amount=%v: expected 200, got %d: %s", amount, rr.Code, rr.Body.String())
	}
	var er EvaluateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &er); err != nil {
		t.Fatalf("decode evaluation response: %v", err)
	}
	return er
}

func decodeStringMap(t *testing.T, body []byte) map[string]string {
	t.Helper()
	m := map[string]string{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode string map: %v (body=%s)", err, body)
	}
	return m
}

func decodeAnyMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode any map: %v (body=%s)", err, body)
	}
	return m
}

// TestZZ_PutDisableReferencedRuleReturns409AndErrorNamesTypology asserts the
// core fix: disabling (enabled:false) a rule referenced by a loaded typology
// is refused with 409, the error names both the rule and the typology, and
// points the operator at the ?force=true opt-in. DELETE on the same rule must
// remain 409, and the refused disable must leave the rule active.
func TestZZ_PutDisableReferencedRuleReturns409AndErrorNamesTypology(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("ref-rule-zz", "amount > 100.0", 1.0, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("ref-typ-zz", "Ref Typ", 0.5, []map[string]any{
		{"ruleId": "ref-rule-zz", "weight": 1.0},
	}))

	rr := doUpdateRule(t, server, zzTenant, "ref-rule-zz", ruleUpdateBody("ref-rule-zz", "amount > 100.0", 1.0, false), false)
	if rr.Code != http.StatusConflict {
		t.Fatalf("disable referenced rule without force: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeStringMap(t, rr.Body.Bytes())
	if !strings.Contains(body["error"], "ref-rule-zz") {
		t.Fatalf("expected error to name the rule, got %q", body["error"])
	}
	if !strings.Contains(body["error"], "ref-typ-zz") {
		t.Fatalf("expected error to name the referencing typology, got %q", body["error"])
	}
	if !strings.Contains(body["error"], "force=true") {
		t.Fatalf("expected error to mention the ?force=true opt-in, got %q", body["error"])
	}

	// DELETE must still be refused with 409 for parity with the PUT guard.
	if drr := doDeleteRule(t, server, zzTenant, "ref-rule-zz"); drr.Code != http.StatusConflict {
		t.Fatalf("DELETE referenced rule: expected 409 (parity), got %d: %s", drr.Code, drr.Body.String())
	}

	// The refused disable must not have mutated the rule: it stays active.
	getReq := httptest.NewRequest(http.MethodGet, "/rules/ref-rule-zz", nil)
	getReq.Header.Set("X-Tenant-ID", zzTenant)
	getResp := httptest.NewRecorder()
	server.Router().ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("expected refused disable to leave the rule present (200), got %d: %s", getResp.Code, getResp.Body.String())
	}
	rule := decodeAnyMap(t, getResp.Body.Bytes())
	if rule["id"] != "ref-rule-zz" {
		t.Fatalf("expected rule id ref-rule-zz, got %v", rule["id"])
	}
}

// TestZZ_PutDisableReferencedRuleForceReturns200WithWarnings asserts the
// ?force=true opt-in path: the rule is disabled (engine reloaded without it)
// and the response carries a warnings array naming the referencing typology.
func TestZZ_PutDisableReferencedRuleForceReturns200WithWarnings(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("force-rule-zz", "amount > 100.0", 1.0, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("force-typ-zz", "Force Typ", 0.5, []map[string]any{
		{"ruleId": "force-rule-zz", "weight": 1.0},
	}))

	rr := doUpdateRule(t, server, zzTenant, "force-rule-zz", ruleUpdateBody("force-rule-zz", "amount > 100.0", 1.0, false), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("force disable: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeAnyMap(t, rr.Body.Bytes())
	warnings, ok := body["warnings"].([]any)
	if !ok || len(warnings) != 1 {
		t.Fatalf("expected warnings array with one entry, got %v", body["warnings"])
	}
	warn, _ := warnings[0].(string)
	if !strings.Contains(warn, "force-rule-zz") || !strings.Contains(warn, "force-typ-zz") {
		t.Fatalf("expected warning to name rule and typology, got %q", warn)
	}

	// The rule must actually have been disabled (saved + reloaded without it).
	listReq := httptest.NewRequest(http.MethodGet, "/rules", nil)
	listReq.Header.Set("X-Tenant-ID", zzTenant)
	listResp := httptest.NewRecorder()
	server.Router().ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list rules: expected 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode rules list: %v", err)
	}
	if listed.Count != 0 {
		t.Fatalf("expected disabled rule to be dropped from the active engine (count 0), got %d", listed.Count)
	}

	// Re-enabling the rule must also work and carry no warnings.
	reEnable := doUpdateRule(t, server, zzTenant, "force-rule-zz", ruleUpdateBody("force-rule-zz", "amount > 100.0", 1.0, true), false)
	if reEnable.Code != http.StatusOK {
		t.Fatalf("re-enable rule: expected 200, got %d: %s", reEnable.Code, reEnable.Body.String())
	}
	if reBody := decodeAnyMap(t, reEnable.Body.Bytes()); reBody["warnings"] != nil {
		t.Fatalf("expected no warnings when enabling a referenced rule, got %v", reBody["warnings"])
	}
}

// TestZZ_PutDisableNonReferencedRuleReturns200WithoutWarnings asserts the
// guard only fires for rules referenced by loaded typologies. Disabling a
// rule no typology references succeeds with 200 and no warnings.
func TestZZ_PutDisableNonReferencedRuleReturns200WithoutWarnings(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("free-rule-zz", "amount > 100.0", 1.0, true))
	// A typology referencing a different rule.
	mustCreateRule(t, server, zzTenant, ruleCreateBody("other-rule-zz", "amount > 50.0", 1.0, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("other-typ-zz", "Other Typ", 0.5, []map[string]any{
		{"ruleId": "other-rule-zz", "weight": 1.0},
	}))

	rr := doUpdateRule(t, server, zzTenant, "free-rule-zz", ruleUpdateBody("free-rule-zz", "amount > 100.0", 1.0, false), false)
	if rr.Code != http.StatusOK {
		t.Fatalf("disable non-referenced rule: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if body := decodeAnyMap(t, rr.Body.Bytes()); body["warnings"] != nil {
		t.Fatalf("expected no warnings for non-referenced rule disable, got %v", body["warnings"])
	}
}

// TestZZ_PutEnableReferencedRuleNoGuard asserts the guard is scoped to the
// disable case: updating a referenced rule with enabled:true never returns
// 409 and carries no warnings, since enabling does not drop the rule.
func TestZZ_PutEnableReferencedRuleNoGuard(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("en-rule-zz", "amount > 100.0", 1.0, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("en-typ-zz", "En Typ", 0.5, []map[string]any{
		{"ruleId": "en-rule-zz", "weight": 1.0},
	}))

	rr := doUpdateRule(t, server, zzTenant, "en-rule-zz", ruleUpdateBody("en-rule-zz", "amount > 200.0", 1.0, true), false)
	if rr.Code != http.StatusOK {
		t.Fatalf("update (enabled) referenced rule: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if body := decodeAnyMap(t, rr.Body.Bytes()); body["warnings"] != nil {
		t.Fatalf("expected no warnings when enabling/updating a referenced rule, got %v", body["warnings"])
	}
}

// TestZZ_PutDisableReferencedRuleInvalidCelStill400 asserts input validation
// (CEL compile check) precedes the referential-integrity guard: an invalid
// expression yields 400 even when the rule is typology-referenced.
func TestZZ_PutDisableReferencedRuleInvalidCelStill400(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("cel-rule-zz", "amount > 100.0", 1.0, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("cel-typ-zz", "Cel Typ", 0.5, []map[string]any{
		{"ruleId": "cel-rule-zz", "weight": 1.0},
	}))

	rr := doUpdateRule(t, server, zzTenant, "cel-rule-zz", ruleUpdateBody("cel-rule-zz", "this is not valid cel !!!", 1.0, false), false)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid CEL on disable: expected 400 (validation precedes guard), got %d: %s", rr.Code, rr.Body.String())
	}
	if body := decodeStringMap(t, rr.Body.Bytes()); !strings.Contains(body["error"], "invalid CEL expression") {
		t.Fatalf("expected CEL error, got %q", body["error"])
	}
}

// TestZZ_PutDisableRuleReferencedByMultipleTypologies asserts the guard
// aggregates every referencing typology: the 409 error names all of them, and
// the forced 200 warnings array lists all of them.
func TestZZ_PutDisableRuleReferencedByMultipleTypologies(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("multi-rule-zz", "amount > 100.0", 1.0, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("multi-typ-a-zz", "Multi A", 0.5, []map[string]any{
		{"ruleId": "multi-rule-zz", "weight": 1.0},
	}))
	mustCreateTypology(t, server, zzTenant, typologyBody("multi-typ-b-zz", "Multi B", 0.5, []map[string]any{
		{"ruleId": "multi-rule-zz", "weight": 1.0},
	}))

	rr := doUpdateRule(t, server, zzTenant, "multi-rule-zz", ruleUpdateBody("multi-rule-zz", "amount > 100.0", 1.0, false), false)
	if rr.Code != http.StatusConflict {
		t.Fatalf("disable multi-referenced rule: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	errBody := decodeStringMap(t, rr.Body.Bytes())
	if !strings.Contains(errBody["error"], "multi-typ-a-zz") || !strings.Contains(errBody["error"], "multi-typ-b-zz") {
		t.Fatalf("expected 409 error to name both typologies, got %q", errBody["error"])
	}

	forceRR := doUpdateRule(t, server, zzTenant, "multi-rule-zz", ruleUpdateBody("multi-rule-zz", "amount > 100.0", 1.0, false), true)
	if forceRR.Code != http.StatusOK {
		t.Fatalf("force disable multi-referenced rule: expected 200, got %d: %s", forceRR.Code, forceRR.Body.String())
	}
	body := decodeAnyMap(t, forceRR.Body.Bytes())
	warnings, ok := body["warnings"].([]any)
	if !ok || len(warnings) != 1 {
		t.Fatalf("expected a single warning string aggregating both typologies, got %v", body["warnings"])
	}
	warn, _ := warnings[0].(string)
	if !strings.Contains(warn, "multi-typ-a-zz") || !strings.Contains(warn, "multi-typ-b-zz") {
		t.Fatalf("expected warning to name both typologies, got %q", warn)
	}
}

// TestZZ_PutDisableReferencedRuleRequiresAdminToken asserts the admin-token
// middleware still gates the PUT path before the handler/guard runs.
func TestZZ_PutDisableReferencedRuleRequiresAdminToken(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("auth-rule-zz", "amount > 100.0", 1.0, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("auth-typ-zz", "Auth Typ", 0.5, []map[string]any{
		{"ruleId": "auth-rule-zz", "weight": 1.0},
	}))

	b, _ := json.Marshal(ruleUpdateBody("auth-rule-zz", "amount > 100.0", 1.0, false))
	req := httptest.NewRequest(http.MethodPut, "/rules/auth-rule-zz", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", zzTenant)
	// No admin token.
	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("PUT-disable without admin token: expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestZZ_E2ENoDecisionFlipAfterPutDisableWithoutForce reproduces the reported
// compliance-mode decision flip: two pivotal rules (weights 0.5/0.5) in a
// typology with alertThreshold 0.7 both fire on amount=9000 (score 1.0 >= 0.7
// -> ALRT). Disabling a pivotal rule via PUT must now be refused (409) so the
// decision stays ALRT instead of silently flipping to NALT (0.5 < 0.7).
func TestZZ_E2ENoDecisionFlipAfterPutDisableWithoutForce(t *testing.T) {
	server, cleanup := createComplianceServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("rule-A-zz", "amount >= 9000.0 && amount < 10000.0", 0.5, true))
	mustCreateRule(t, server, zzTenant, ruleCreateBody("rule-B-zz", "amount >= 1000.0", 0.5, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("flip-typ-zz", "Flip Typ", 0.7, []map[string]any{
		{"ruleId": "rule-A-zz", "weight": 0.5},
		{"ruleId": "rule-B-zz", "weight": 0.5},
	}))

	before := doEvaluate(t, server, zzTenant, 9000.0)
	if before.Status != domain.StatusAlert {
		t.Fatalf("baseline: expected ALRT, got %s (score %.2f)", before.Status, before.Score)
	}

	rr := doUpdateRule(t, server, zzTenant, "rule-A-zz", ruleUpdateBody("rule-A-zz", "amount >= 9000.0 && amount < 10000.0", 0.5, false), false)
	if rr.Code != http.StatusConflict {
		t.Fatalf("disable pivotal referenced rule without force: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}

	after := doEvaluate(t, server, zzTenant, 9000.0)
	if after.Status != domain.StatusAlert {
		t.Fatalf("DECISION FLIP after refused disable: expected ALRT (bug fixed), got %s (score %.2f -> %.2f)",
			after.Status, before.Score, after.Score)
	}
	if after.Score != before.Score {
		t.Fatalf("expected score unchanged after refused disable, got %.2f -> %.2f", before.Score, after.Score)
	}
}

// TestZZ_E2EObjectFlipWithForceAndWarnings asserts the ?force=true opt-in
// path end-to-end: the pivotal rule is disabled, the response surfaces a
// warning naming the typology, and the decision flips to NALT because the
// operator explicitly acknowledged the dependency.
func TestZZ_E2EObjectFlipWithForceAndWarnings(t *testing.T) {
	server, cleanup := createComplianceServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("f-rule-A-zz", "amount >= 9000.0 && amount < 10000.0", 0.5, true))
	mustCreateRule(t, server, zzTenant, ruleCreateBody("f-rule-B-zz", "amount >= 1000.0", 0.5, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("f-typ-zz", "Force Typ", 0.7, []map[string]any{
		{"ruleId": "f-rule-A-zz", "weight": 0.5},
		{"ruleId": "f-rule-B-zz", "weight": 0.5},
	}))

	before := doEvaluate(t, server, zzTenant, 9000.0)
	if before.Status != domain.StatusAlert {
		t.Fatalf("baseline: expected ALRT, got %s (score %.2f)", before.Status, before.Score)
	}

	rr := doUpdateRule(t, server, zzTenant, "f-rule-A-zz", ruleUpdateBody("f-rule-A-zz", "amount >= 9000.0 && amount < 10000.0", 0.5, false), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("force disable pivotal rule: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeAnyMap(t, rr.Body.Bytes())
	warnings, ok := body["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("expected warnings array naming the typology, got %v", body["warnings"])
	}
	if warn, _ := warnings[0].(string); !strings.Contains(warn, "f-typ-zz") {
		t.Fatalf("expected warning to name the typology, got %q", warn)
	}

	after := doEvaluate(t, server, zzTenant, 9000.0)
	if after.Status != domain.StatusNoAlert {
		t.Fatalf("after force disable: expected NALT (opted-in flip, 0.5 < 0.7), got %s (score %.2f)", after.Status, after.Score)
	}
	if after.Score >= 0.7 {
		t.Fatalf("expected depleted score below threshold 0.7, got %.2f", after.Score)
	}
}

// TestZZ_E2ENoFlipWhenRuleNotPivotal characterizes the pivotality boundary:
// with the same rule weights but a lower alertThreshold (0.3), disabling one
// rule via force leaves the remaining contribution (0.5) above threshold, so
// the decision stays ALRT. The guard still fires (409) without force.
func TestZZ_E2ENoFlipWhenRuleNotPivotal(t *testing.T) {
	server, cleanup := createComplianceServer(t)
	defer cleanup()

	mustCreateRule(t, server, zzTenant, ruleCreateBody("nf-rule-A-zz", "amount >= 9000.0 && amount < 10000.0", 0.5, true))
	mustCreateRule(t, server, zzTenant, ruleCreateBody("nf-rule-B-zz", "amount >= 1000.0", 0.5, true))
	mustCreateTypology(t, server, zzTenant, typologyBody("nf-typ-zz", "NF Typ", 0.3, []map[string]any{
		{"ruleId": "nf-rule-A-zz", "weight": 0.5},
		{"ruleId": "nf-rule-B-zz", "weight": 0.5},
	}))

	before := doEvaluate(t, server, zzTenant, 9000.0)
	if before.Status != domain.StatusAlert {
		t.Fatalf("baseline: expected ALRT, got %s (score %.2f)", before.Status, before.Score)
	}

	// Without force, the disable is refused even though the rule is not pivotal.
	if rr := doUpdateRule(t, server, zzTenant, "nf-rule-A-zz", ruleUpdateBody("nf-rule-A-zz", "amount >= 9000.0 && amount < 10000.0", 0.5, false), false); rr.Code != http.StatusConflict {
		t.Fatalf("disable non-pivotal referenced rule without force: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}

	// With force the rule is disabled; the remaining score (0.5) is still >= 0.3.
	forceRR := doUpdateRule(t, server, zzTenant, "nf-rule-A-zz", ruleUpdateBody("nf-rule-A-zz", "amount >= 9000.0 && amount < 10000.0", 0.5, false), true)
	if forceRR.Code != http.StatusOK {
		t.Fatalf("force disable non-pivotal rule: expected 200, got %d: %s", forceRR.Code, forceRR.Body.String())
	}

	after := doEvaluate(t, server, zzTenant, 9000.0)
	if after.Status != domain.StatusAlert {
		t.Fatalf("after force disable of non-pivotal rule: expected ALRT (0.5 >= 0.3), got %s (score %.2f)", after.Status, after.Score)
	}
}

// TestZZ_E2EFlipOnShippedFatfStructuringTypology reproduces the flip on the
// shipped FATF structuring typology (alertThreshold 0.5; weights
// structuring-001 0.5, round-amount-001 0.25, velocity-001 0.25) and the
// shipped rule expressions. A transaction of amount=9000 scores 0.75 (>= 0.5
// -> ALRT). Without force, disabling structuring-001 is refused (409) so the
// decision stays ALRT. With force it is disabled, the depleted score 0.25
// (< 0.5) flips the decision to NALT, and the warning names the typology.
func TestZZ_E2EFlipOnShippedFatfStructuringTypology(t *testing.T) {
	server, cleanup := createComplianceServer(t)
	defer cleanup()

	reviewBands := zzBandsReview()
	mustCreateRule(t, server, zzTenant, ruleCreateBodyBands("structuring-001", "amount >= 9000.0 && amount < 10000.0", 0.6, true, reviewBands))
	mustCreateRule(t, server, zzTenant, ruleCreateBodyBands("round-amount-001", "amount >= 1000.0 && amount == double(int(amount / 1000.0)) * 1000.0", 0.2, true, reviewBands))
	mustCreateRule(t, server, zzTenant, ruleCreateBodyBands("velocity-001", "velocity_count > 5", 0.6, true, reviewBands))
	mustCreateTypology(t, server, zzTenant, typologyBody("typology-structuring", "Structuring (Smurfing)", 0.5, []map[string]any{
		{"ruleId": "structuring-001", "weight": 0.5},
		{"ruleId": "round-amount-001", "weight": 0.25},
		{"ruleId": "velocity-001", "weight": 0.25},
	}))

	// velocity-001 scores 0.0 (no velocity getter wired), so 0.5 + 0.25 = 0.75.
	before := doEvaluate(t, server, zzTenant, 9000.0)
	if before.Status != domain.StatusAlert {
		t.Fatalf("shipped baseline: expected ALRT (score %.2f >= 0.5), got %s", before.Score, before.Status)
	}
	if before.Score < 0.749 || before.Score > 0.751 {
		t.Fatalf("shipped baseline: expected score ~0.75, got %.2f", before.Score)
	}

	rr := doUpdateRule(t, server, zzTenant, "structuring-001", ruleUpdateBody("structuring-001", "amount >= 9000.0 && amount < 10000.0", 0.6, false), false)
	if rr.Code != http.StatusConflict {
		t.Fatalf("disable shipped structuring-001 without force: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(decodeStringMap(t, rr.Body.Bytes())["error"], "typology-structuring") {
		t.Fatalf("expected 409 to name typology-structuring, got %s", rr.Body.String())
	}

	// Refused disable: decision stays ALRT (bug fixed on shipped config).
	if after := doEvaluate(t, server, zzTenant, 9000.0); after.Status != domain.StatusAlert {
		t.Fatalf("shipped: after refused disable expected ALRT, got %s (score %.2f)", after.Status, after.Score)
	}

	// Force the disable: depleted score 0.25 < 0.5 flips to NALT with a warning.
	forceRR := doUpdateRule(t, server, zzTenant, "structuring-001", ruleUpdateBody("structuring-001", "amount >= 9000.0 && amount < 10000.0", 0.6, false), true)
	if forceRR.Code != http.StatusOK {
		t.Fatalf("force disable shipped structuring-001: expected 200, got %d: %s", forceRR.Code, forceRR.Body.String())
	}
	if warn, _ := decodeAnyMap(t, forceRR.Body.Bytes())["warnings"].([]any)[0].(string); !strings.Contains(warn, "typology-structuring") {
		t.Fatalf("expected forced warning to name typology-structuring, got %s", forceRR.Body.String())
	}
	after := doEvaluate(t, server, zzTenant, 9000.0)
	if after.Status != domain.StatusNoAlert {
		t.Fatalf("shipped: after force disable expected NALT (0.25 < 0.5), got %s (score %.2f)", after.Status, after.Score)
	}
	if after.Score < 0.249 || after.Score > 0.251 {
		t.Fatalf("shipped: after force disable expected score ~0.25, got %.2f", after.Score)
	}
}
