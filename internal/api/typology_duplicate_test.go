package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
)

// dupRulesServer builds a persistent server with two loaded rules, rule-a and
// rule-b, so duplicate-ruleId validation can reference real rule IDs the
// engine recognizes.
func dupRulesServer(t *testing.T) (*Server, func()) {
	t.Helper()
	engine, _ := rules.NewEngine(nil, 5)
	for _, id := range []string{"rule-a", "rule-b"} {
		if err := engine.LoadRule(&domain.RuleConfig{
			ID:         id,
			Name:       id,
			Expression: "1 == 1",
			Weight:     1.0,
			Enabled:    true,
		}); err != nil {
			t.Fatalf("failed to load test rule %s: %v", id, err)
		}
	}
	return createPersistentTestServerWithEngine(t, engine)
}

func typologyDuplicateBody(id, name string, threshold float64, ruleWeights []map[string]any) map[string]any {
	return map[string]any{
		"id":             id,
		"name":           name,
		"alertThreshold": threshold,
		"enabled":        true,
		"rules":          ruleWeights,
	}
}

func postTypology(t *testing.T, s *Server, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/typologies", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-dup")
	setAdminAuth(req)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	return rr
}

func putTypology(t *testing.T, s *Server, id string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/typologies/"+id, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-dup")
	setAdminAuth(req)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	return rr
}

func activeTypologyCount(t *testing.T, s *Server) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/typologies", nil)
	req.Header.Set("X-Tenant-ID", "tenant-dup")
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list typologies: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var listed struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode typology list: %v", err)
	}
	return listed.Count
}

// TestTypologyCreateRejectsDuplicateRuleIDs confirms POST /typologies returns
// 400 when the same ruleId appears twice, names the duplicated ruleId, and does
// not persist or load the typology.
func TestTypologyCreateRejectsDuplicateRuleIDs(t *testing.T) {
	server, cleanup := dupRulesServer(t)
	defer cleanup()

	body := typologyDuplicateBody("dup-create", "Dup Create", 0.5, []map[string]any{
		{"ruleId": "rule-a", "weight": 0.5},
		{"ruleId": "rule-a", "weight": 0.5},
	})
	rr := postTypology(t, server, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate ruleId, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "duplicate ruleId") {
		t.Fatalf("expected error to mention duplicate ruleId, got %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "rule-a") {
		t.Fatalf("expected error to name the duplicated ruleId rule-a, got %s", rr.Body.String())
	}

	// The rejected request must not have created or loaded a typology.
	if got := activeTypologyCount(t, server); got != 0 {
		t.Fatalf("rejected create must not load a typology, got count %d", got)
	}
	getReq := httptest.NewRequest(http.MethodGet, "/typologies/dup-create", nil)
	getReq.Header.Set("X-Tenant-ID", "tenant-dup")
	getRR := httptest.NewRecorder()
	server.Router().ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusNotFound {
		t.Fatalf("expected GET for rejected typology to be 404, got %d: %s", getRR.Code, getRR.Body.String())
	}
}

// TestTypologyCreateRejectsDuplicateRuleIDsAfterTrim confirms two ruleId values
// that differ only by surrounding whitespace (" rule-a " and "rule-a") are
// detected as duplicates after the shared TrimSpace, because validation tracks
// the trimmed ID.
func TestTypologyCreateRejectsDuplicateRuleIDsAfterTrim(t *testing.T) {
	server, cleanup := dupRulesServer(t)
	defer cleanup()

	body := typologyDuplicateBody("dup-trim", "Dup Trim", 0.5, []map[string]any{
		{"ruleId": " rule-a ", "weight": 0.5},
		{"ruleId": "rule-a", "weight": 0.5},
	})
	rr := postTypology(t, server, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for trimmed-duplicate ruleId, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "duplicate ruleId") {
		t.Fatalf("expected error to mention duplicate ruleId, got %s", rr.Body.String())
	}
	if got := activeTypologyCount(t, server); got != 0 {
		t.Fatalf("rejected create must not load a typology, got count %d", got)
	}
}

// TestTypologyCreateRejectsDuplicateMaskedByWeightSum confirms a duplicate is
// rejected even when the weights still sum to 1.0, which previously silenced
// the only check (the weight-sum warning) and let the bad payload save.
func TestTypologyCreateRejectsDuplicateMaskedByWeightSum(t *testing.T) {
	server, cleanup := dupRulesServer(t)
	defer cleanup()

	body := typologyDuplicateBody("dup-masked", "Dup Masked", 0.5, []map[string]any{
		{"ruleId": "rule-a", "weight": 0.5},
		{"ruleId": "rule-b", "weight": 0.25},
		{"ruleId": "rule-b", "weight": 0.25},
	})
	rr := postTypology(t, server, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for weight-sum-masked duplicate, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "duplicate ruleId") || !strings.Contains(rr.Body.String(), "rule-b") {
		t.Fatalf("expected error to name duplicate ruleId rule-b, got %s", rr.Body.String())
	}
	if got := activeTypologyCount(t, server); got != 0 {
		t.Fatalf("rejected create must not load a typology, got count %d", got)
	}
}

// TestTypologyUpdateRejectsDuplicateRuleIDs confirms PUT /typologies/{id}
// returns 400 for a duplicate ruleId and leaves the existing typology intact.
func TestTypologyUpdateRejectsDuplicateRuleIDs(t *testing.T) {
	server, cleanup := dupRulesServer(t)
	defer cleanup()

	// Create a valid typology with unique rule IDs.
	createBody := typologyDuplicateBody("dup-update", "Dup Update", 0.5, []map[string]any{
		{"ruleId": "rule-a", "weight": 0.5},
		{"ruleId": "rule-b", "weight": 0.5},
	})
	if rr := postTypology(t, server, createBody); rr.Code != http.StatusCreated {
		t.Fatalf("setup create: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := activeTypologyCount(t, server); got != 1 {
		t.Fatalf("setup: expected active count 1, got %d", got)
	}

	// PUT with a duplicate ruleId must be rejected.
	updateBody := map[string]any{
		"name":           "Dup Update",
		"alertThreshold": 0.5,
		"enabled":        true,
		"rules": []map[string]any{
			{"ruleId": "rule-a", "weight": 0.5},
			{"ruleId": "rule-a", "weight": 0.5},
		},
	}
	rr := putTypology(t, server, "dup-update", updateBody)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate ruleId on update, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "duplicate ruleId") || !strings.Contains(rr.Body.String(), "rule-a") {
		t.Fatalf("expected error to name duplicate ruleId rule-a, got %s", rr.Body.String())
	}

	// The rejected update must not change the active count or the stored rules.
	if got := activeTypologyCount(t, server); got != 1 {
		t.Fatalf("rejected update must not change active count, got %d", got)
	}
	getReq := httptest.NewRequest(http.MethodGet, "/typologies/dup-update", nil)
	getReq.Header.Set("X-Tenant-ID", "tenant-dup")
	getRR := httptest.NewRecorder()
	server.Router().ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("GET after rejected update: expected 200, got %d: %s", getRR.Code, getRR.Body.String())
	}
	// GetTypology returns the bare typology object (not wrapped in a "typology" key).
	var got struct {
		Rules []struct {
			RuleID string `json:"ruleId"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode typology: %v", err)
	}
	if len(got.Rules) != 2 {
		t.Fatalf("expected original 2 rules intact after rejected update, got %d", len(got.Rules))
	}
}

// TestTypologyCreateAcceptsUniqueRuleIDs is a regression guard: a typology with
// distinct ruleId values must still be accepted after the duplicate check.
func TestTypologyCreateAcceptsUniqueRuleIDs(t *testing.T) {
	server, cleanup := dupRulesServer(t)
	defer cleanup()

	body := typologyDuplicateBody("unique-create", "Unique Create", 0.5, []map[string]any{
		{"ruleId": "rule-a", "weight": 0.5},
		{"ruleId": "rule-b", "weight": 0.5},
	})
	rr := postTypology(t, server, body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected unique ruleIds to be accepted (201), got %d: %s", rr.Code, rr.Body.String())
	}
	if got := activeTypologyCount(t, server); got != 1 {
		t.Fatalf("expected active count 1 after valid create, got %d", got)
	}
}
