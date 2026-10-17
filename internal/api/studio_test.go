package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

func createStudioTestServer(t *testing.T, studioEnabled bool) *Server {
	t.Helper()
	engine, err := rules.NewEngine(nil, 5)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	cfg := domain.ServerConfig{Host: "localhost", Port: 8080, Studio: studioEnabled}
	return NewServer(cfg, nil, nil, nil, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), "test-v1", domain.ModeDetection)
}

func TestStudioRoutes(t *testing.T) {
	t.Run("ServesIndexWithoutTenant", func(t *testing.T) {
		srv := createStudioTestServer(t, true)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/studio/", nil))

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "<title>Osprey Studio</title>") {
			t.Fatal("expected the studio index page")
		}
		csp := w.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Fatalf("expected a restrictive CSP, got %q", csp)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("expected X-Content-Type-Options: nosniff")
		}
	})

	t.Run("ServesAssets", func(t *testing.T) {
		srv := createStudioTestServer(t, true)
		for _, path := range []string{"/studio/app.js", "/studio/studio.css"} {
			w := httptest.NewRecorder()
			srv.Router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("%s: expected 200, got %d", path, w.Code)
			}
		}
	})

	t.Run("RedirectsBarePath", func(t *testing.T) {
		srv := createStudioTestServer(t, true)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/studio", nil))
		if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "/studio/" {
			t.Fatalf("expected redirect to /studio/, got %d %q", w.Code, w.Header().Get("Location"))
		}
	})

	t.Run("OffWhenDisabled", func(t *testing.T) {
		srv := createStudioTestServer(t, false)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/studio/", nil))
		if w.Code == http.StatusOK {
			t.Fatal("expected the studio to be off when cfg.Studio is false")
		}
	})
}

func TestValidateExpression(t *testing.T) {
	srv := createStudioTestServer(t, false)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantValid  string
		wantErr    string
	}{
		{"ValidBool", `{"expression":"amount > 10000.0"}`, http.StatusOK, `"valid":true`, ""},
		{"IntDoubleMismatch", `{"expression":"amount > 10000"}`, http.StatusOK, `"valid":false`, "no matching overload"},
		{"SyntaxError", `{"expression":"amount >"}`, http.StatusOK, `"valid":false`, "Syntax error"},
		{"WrongOutputType", `{"expression":"debtor_id"}`, http.StatusOK, `"valid":false`, "must return bool, int, or double"},
		{"Empty", `{"expression":"  "}`, http.StatusBadRequest, "", "expression is required"},
		{"UnknownField", `{"expression":"true","x":1}`, http.StatusBadRequest, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/rules/validate", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(TenantIDHeader, "tenant-a")
			w := httptest.NewRecorder()
			srv.Router().ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
			if tt.wantValid != "" && !strings.Contains(w.Body.String(), tt.wantValid) {
				t.Fatalf("expected %s in %s", tt.wantValid, w.Body.String())
			}
			if tt.wantErr != "" && !strings.Contains(w.Body.String(), tt.wantErr) {
				t.Fatalf("expected %q in %s", tt.wantErr, w.Body.String())
			}
		})
	}

	t.Run("DoesNotLoadTheRule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/rules/validate", strings.NewReader(`{"expression":"true"}`))
		req.Header.Set(TenantIDHeader, "tenant-a")
		srv.Router().ServeHTTP(httptest.NewRecorder(), req)

		w := httptest.NewRecorder()
		get := httptest.NewRequest(http.MethodGet, "/rules", nil)
		get.Header.Set(TenantIDHeader, "tenant-a")
		srv.Router().ServeHTTP(w, get)
		if !strings.Contains(w.Body.String(), `"count":0`) {
			t.Fatalf("validate must not load a rule, got %s", w.Body.String())
		}
	})
}
