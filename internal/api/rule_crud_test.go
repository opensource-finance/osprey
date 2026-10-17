package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

// postRuleJSON sends a raw JSON rule body to POST /rules and returns the
// response recorder. It does not assume any field defaults so callers can
// probe boundary behavior such as weight 0 or omitted weight.
func postRuleJSON(t *testing.T, server *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-weight")
	setAdminAuth(req)
	resp := httptest.NewRecorder()
	server.Router().ServeHTTP(resp, req)
	return resp
}

// getRuleJSON fetches a rule from the active engine and returns its decoded
// body. Fails the test if the rule is not found or the body is not an object.
func getRuleJSON(t *testing.T, server *Server, id string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/rules/"+id, nil)
	req.Header.Set("X-Tenant-ID", "tenant-weight")
	resp := httptest.NewRecorder()
	server.Router().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("get rule %s: expected 200, got %d: %s", id, resp.Code, resp.Body.String())
	}
	var rule map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &rule); err != nil {
		t.Fatalf("decode rule %s: %v", id, err)
	}
	return rule
}

// TestRuleWeightZeroSemantics covers the zero-weight bug fix at the API boundary:
// an explicit weight of 0 is accepted and persisted verbatim, while an omitted
// (or null) weight defaults to 1.0 so callers that rely on the implicit default
// do not silently change behavior.
func TestRuleWeightZeroSemantics(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	t.Run("ExplicitZeroAcceptedAndPersistedAsZero", func(t *testing.T) {
		body := `{"id":"zero-rule","name":"Zero","expression":"amount > 1.0","weight":0,"enabled":true}`
		resp := postRuleJSON(t, server, body)
		if resp.Code != http.StatusCreated {
			t.Fatalf("expected 201 for explicit weight 0, got %d: %s", resp.Code, resp.Body.String())
		}
		var created struct {
			Rule struct {
				Weight float64 `json:"weight"`
			} `json:"rule"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		if created.Rule.Weight != 0 {
			t.Errorf("create response: expected weight 0, got %v", created.Rule.Weight)
		}
		if got := getRuleJSON(t, server, "zero-rule")["weight"]; got != 0.0 {
			t.Errorf("active engine: expected weight 0, got %v", got)
		}
	})

	t.Run("OmittedWeightDefaultsToOne", func(t *testing.T) {
		// No "weight" field at all. The JSON zero-value would be 0, but the
		// pointer field lets the handler default it to 1.0.
		body := `{"id":"default-rule","name":"Default","expression":"amount > 1.0","enabled":true}`
		resp := postRuleJSON(t, server, body)
		if resp.Code != http.StatusCreated {
			t.Fatalf("expected 201 for omitted weight, got %d: %s", resp.Code, resp.Body.String())
		}
		if got := getRuleJSON(t, server, "default-rule")["weight"]; got != 1.0 {
			t.Errorf("expected omitted weight to default to 1.0, got %v", got)
		}
	})

	t.Run("NullWeightDefaultsToOne", func(t *testing.T) {
		body := `{"id":"null-rule","name":"Null","expression":"amount > 1.0","weight":null,"enabled":true}`
		resp := postRuleJSON(t, server, body)
		if resp.Code != http.StatusCreated {
			t.Fatalf("expected 201 for null weight, got %d: %s", resp.Code, resp.Body.String())
		}
		if got := getRuleJSON(t, server, "null-rule")["weight"]; got != 1.0 {
			t.Errorf("expected null weight to default to 1.0, got %v", got)
		}
	})

	t.Run("NegativeWeightRejected", func(t *testing.T) {
		body := `{"id":"neg-rule","name":"Neg","expression":"amount > 1.0","weight":-0.1,"enabled":true}`
		resp := postRuleJSON(t, server, body)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for negative weight, got %d: %s", resp.Code, resp.Body.String())
		}
		if !strings.Contains(resp.Body.String(), "weight must be between 0 and 1") {
			t.Fatalf("expected weight validation message, got %s", resp.Body.String())
		}
	})

	t.Run("UpdateToExplicitZeroPersisted", func(t *testing.T) {
		createRuleForTest(t, server, "update-zero", "amount > 1.0")

		body := `{"name":"Updated","expression":"amount > 2.0","weight":0,"enabled":true}`
		req := httptest.NewRequest(http.MethodPut, "/rules/update-zero", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-weight")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200 for update to weight 0, got %d: %s", resp.Code, resp.Body.String())
		}
		if got := getRuleJSON(t, server, "update-zero")["weight"]; got != 0.0 {
			t.Errorf("expected updated weight 0, got %v", got)
		}
	})

	t.Run("UpdateOmittedWeightDefaultsToOne", func(t *testing.T) {
		createRuleForTest(t, server, "update-default", "amount > 1.0")

		// Omit weight on update; the handler defaults it to 1.0 rather than
		// silently zeroing the rule's contribution.
		body := `{"name":"Updated","expression":"amount > 2.0","enabled":true}`
		req := httptest.NewRequest(http.MethodPut, "/rules/update-default", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-weight")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200 for update with omitted weight, got %d: %s", resp.Code, resp.Body.String())
		}
		if got := getRuleJSON(t, server, "update-default")["weight"]; got != 1.0 {
			t.Errorf("expected omitted update weight to default to 1.0, got %v", got)
		}
	})
}

// TestRuleZeroWeightDoesNotInflateDetectionScore is an end-to-end check in
// detection mode. A high-scoring rule saved with weight 0 must not pull the
// aggregate score up or push the transaction over the alert threshold. With
// the old clamp (weight 0 -> 1.0) this scenario produced a false ALRT.
func TestRuleZeroWeightDoesNotInflateDetectionScore(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	// Contributing rule: weighted 1.0, score 0.6 on amount > 50.
	contrib := `{"id":"contrib-rule","name":"Contrib","expression":"amount > 50.0 ? 0.6 : 0.0","weight":1.0,"enabled":true}`
	if resp := postRuleJSON(t, server, contrib); resp.Code != http.StatusCreated {
		t.Fatalf("create contrib rule: expected 201, got %d: %s", resp.Code, resp.Body.String())
	}
	// Monitor-only rule: weighted 0, score 1.0 on amount > 50. It must count
	// toward nothing even though its own score is the maximum.
	monitor := `{"id":"monitor-rule","name":"Monitor","expression":"amount > 50.0 ? 1.0 : 0.0","weight":0.0,"enabled":true}`
	if resp := postRuleJSON(t, server, monitor); resp.Code != http.StatusCreated {
		t.Fatalf("create monitor rule: expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	// Evaluate at amount 100 (both rules fire).
	txBody, _ := json.Marshal(TransactionRequest{
		Type:     "transfer",
		Debtor:   PartyInfo{ID: "d-w", AccountID: "a1"},
		Creditor: PartyInfo{ID: "c-w", AccountID: "a2"},
		Amount:   AmountInfo{Value: 100.0, Currency: "USD"},
	})
	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(txBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-weight")
	resp := httptest.NewRecorder()
	server.Router().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("evaluate: expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var er EvaluateResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &er); err != nil {
		t.Fatalf("decode evaluation: %v", err)
	}

	// With the fix: (0.6*1.0 + 1.0*0.0) / 1.0 = 0.6 < 0.7 -> NALT.
	// With the bug: (0.6 + 1.0) / 2.0 = 0.8 >= 0.7 -> ALRT.
	if er.Status != "NALT" {
		t.Errorf("expected NALT (zero-weight rule must not contribute), got %s (score %.3f)", er.Status, er.Score)
	}
	if er.Score < 0.599 || er.Score > 0.601 {
		t.Errorf("expected score ~0.6, got %.3f", er.Score)
	}

	// Fetch the saved evaluation to prove the zero-weight rule was still
	// evaluated and persisted with its true score and weight.
	getReq := httptest.NewRequest(http.MethodGet, "/evaluations/"+er.EvaluationID, nil)
	getReq.Header.Set("X-Tenant-ID", "tenant-weight")
	getResp := httptest.NewRecorder()
	server.Router().ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get evaluation: expected 200, got %d: %s", getResp.Code, getResp.Body.String())
	}
	var eval domain.Evaluation
	if err := json.Unmarshal(getResp.Body.Bytes(), &eval); err != nil {
		t.Fatalf("decode saved evaluation: %v", err)
	}
	var sawContrib, sawMonitor bool
	for _, r := range eval.RuleResults {
		switch r.RuleID {
		case "contrib-rule":
			sawContrib = true
			if r.Weight != 1.0 || r.Score < 0.599 || r.Score > 0.601 {
				t.Errorf("contrib-rule: expected weight 1.0 score 0.6, got weight %v score %v", r.Weight, r.Score)
			}
		case "monitor-rule":
			sawMonitor = true
			if r.Weight != 0.0 {
				t.Errorf("monitor-rule: expected weight 0.0, got %v", r.Weight)
			}
			if r.Score < 0.999 || r.Score > 1.001 {
				t.Errorf("monitor-rule: expected score 1.0, got %v", r.Score)
			}
		}
	}
	if !sawContrib || !sawMonitor {
		t.Errorf("expected both rules in saved evaluation, got contrib=%v monitor=%v", sawContrib, sawMonitor)
	}
}

// createRule is a small helper that POSTs a rule and asserts it was created.
func createRuleForTest(t *testing.T, server *Server, id, expression string) {
	t.Helper()
	payload := map[string]any{
		"id":         id,
		"name":       id,
		"expression": expression,
		"weight":     1.0,
		"enabled":    true,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-crud")
	setAdminAuth(req)
	resp := httptest.NewRecorder()
	server.Router().ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create rule %s: expected 201, got %d: %s", id, resp.Code, resp.Body.String())
	}
}

func TestUpdateRule(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	createRuleForTest(t, server, "crud-rule", "amount > 100.0")

	t.Run("UpdatesExistingRule", func(t *testing.T) {
		payload := map[string]any{
			"name":       "Updated Name",
			"expression": "amount > 500.0",
			"weight":     1.0,
			"enabled":    true,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/rules/crud-rule", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-crud")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		// Confirm the change is reflected in the active engine.
		getReq := httptest.NewRequest(http.MethodGet, "/rules/crud-rule", nil)
		getReq.Header.Set("X-Tenant-ID", "tenant-crud")
		getResp := httptest.NewRecorder()
		server.Router().ServeHTTP(getResp, getReq)
		if getResp.Code != http.StatusOK {
			t.Fatalf("expected GET 200, got %d: %s", getResp.Code, getResp.Body.String())
		}
		var rule map[string]any
		if err := json.Unmarshal(getResp.Body.Bytes(), &rule); err != nil {
			t.Fatalf("decode rule: %v", err)
		}
		if rule["name"] != "Updated Name" {
			t.Fatalf("expected updated name, got %v", rule["name"])
		}
		if rule["expression"] != "amount > 500.0" {
			t.Fatalf("expected updated expression, got %v", rule["expression"])
		}
	})

	t.Run("RejectsInvalidCEL", func(t *testing.T) {
		payload := map[string]any{
			"name":       "Bad",
			"expression": "this is not valid cel !!!",
			"weight":     1.0,
			"enabled":    true,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/rules/crud-rule", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-crud")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid CEL, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("RequiresEnabled", func(t *testing.T) {
		// Omitting enabled must not silently disable the rule.
		payload := map[string]any{"name": "x", "expression": "amount > 1.0", "weight": 1.0}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/rules/crud-rule", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-crud")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "enabled is required") {
			t.Fatalf("expected 400 'enabled is required', got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("RequiresAdminToken", func(t *testing.T) {
		payload := map[string]any{"name": "x", "expression": "amount > 1.0", "weight": 1.0, "enabled": true}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/rules/crud-rule", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-crud")
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 without admin token, got %d: %s", resp.Code, resp.Body.String())
		}
	})
}

func TestDeleteRule(t *testing.T) {
	server, cleanup := createPersistentTestServer(t)
	defer cleanup()

	createRuleForTest(t, server, "deletable-rule", "amount > 100.0")

	t.Run("DeletesRuleAndRemovesFromEngine", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/rules/deletable-rule", nil)
		req.Header.Set("X-Tenant-ID", "tenant-crud")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		getReq := httptest.NewRequest(http.MethodGet, "/rules/deletable-rule", nil)
		getReq.Header.Set("X-Tenant-ID", "tenant-crud")
		getResp := httptest.NewRecorder()
		server.Router().ServeHTTP(getResp, getReq)
		if getResp.Code != http.StatusNotFound {
			t.Fatalf("expected deleted rule GET to be 404, got %d: %s", getResp.Code, getResp.Body.String())
		}
	})

	t.Run("RefusesDeleteWhenReferencedByTypology", func(t *testing.T) {
		createRuleForTest(t, server, "referenced-rule", "debtor_id == creditor_id")

		// Create a typology that references the rule.
		typPayload := map[string]any{
			"id":             "ref-typology",
			"name":           "Ref Typology",
			"alertThreshold": 0.5,
			"enabled":        true,
			"rules":          []map[string]any{{"ruleId": "referenced-rule", "weight": 1.0}},
		}
		body, _ := json.Marshal(typPayload)
		req := httptest.NewRequest(http.MethodPost, "/typologies", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-crud")
		setAdminAuth(req)
		resp := httptest.NewRecorder()
		server.Router().ServeHTTP(resp, req)
		if resp.Code != http.StatusCreated {
			t.Fatalf("create typology: expected 201, got %d: %s", resp.Code, resp.Body.String())
		}

		// Deleting the referenced rule must be refused.
		delReq := httptest.NewRequest(http.MethodDelete, "/rules/referenced-rule", nil)
		delReq.Header.Set("X-Tenant-ID", "tenant-crud")
		setAdminAuth(delReq)
		delResp := httptest.NewRecorder()
		server.Router().ServeHTTP(delResp, delReq)
		if delResp.Code != http.StatusConflict {
			t.Fatalf("expected 409 for rule referenced by typology, got %d: %s", delResp.Code, delResp.Body.String())
		}
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("DisabledPassesThrough", func(t *testing.T) {
		h := RateLimitMiddleware(0, 0)(okHandler)
		for i := range 5 {
			req := httptest.NewRequest(http.MethodPost, "/evaluate", nil)
			req.Header.Set("X-Tenant-ID", "t1")
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("disabled limiter request %d: expected 200, got %d", i, resp.Code)
			}
		}
	})

	t.Run("ThrottlesPerTenant", func(t *testing.T) {
		h := RateLimitMiddleware(1, 1)(okHandler) // 1 rps, burst 1

		do := func(tenant string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPost, "/evaluate", nil)
			req.Header.Set("X-Tenant-ID", tenant)
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			return resp
		}

		// First request for tenant A consumes the single token.
		if r := do("A"); r.Code != http.StatusOK {
			t.Fatalf("first A request: expected 200, got %d", r.Code)
		}
		// Immediate second request for A is throttled.
		r := do("A")
		if r.Code != http.StatusTooManyRequests {
			t.Fatalf("second A request: expected 429, got %d", r.Code)
		}
		if r.Header().Get("Retry-After") == "" {
			t.Fatalf("expected Retry-After header on 429")
		}
		// A different tenant has its own bucket and is not throttled.
		if r := do("B"); r.Code != http.StatusOK {
			t.Fatalf("first B request: expected 200, got %d", r.Code)
		}
	})
}
