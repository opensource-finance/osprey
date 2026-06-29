package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// createRule is a small helper that POSTs a rule and asserts it was created.
func createRuleForTest(t *testing.T, server *Server, id, expression string) {
	t.Helper()
	payload := map[string]interface{}{
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
		payload := map[string]interface{}{
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
		var rule map[string]interface{}
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
		payload := map[string]interface{}{
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

	t.Run("RequiresAdminToken", func(t *testing.T) {
		payload := map[string]interface{}{"name": "x", "expression": "amount > 1.0", "weight": 1.0, "enabled": true}
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
		typPayload := map[string]interface{}{
			"id":             "ref-typology",
			"name":           "Ref Typology",
			"alertThreshold": 0.5,
			"enabled":        true,
			"rules":          []map[string]interface{}{{"ruleId": "referenced-rule", "weight": 1.0}},
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
		for i := 0; i < 5; i++ {
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
