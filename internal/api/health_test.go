package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opensource-finance/osprey/internal/bus"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// fakeBus is a domain.EventBus stub whose only health-relevant method is Ping.
// The health handler never calls Publish/Subscribe/Close, so the embedded
// interface is left nil without risk of a nil-dereference on other methods.
type fakeBus struct {
	domain.EventBus
	pingErr error
}

func (b *fakeBus) Ping(_ context.Context) error { return b.pingErr }

func createTestServerWithBus(b domain.EventBus) *Server {
	cfg := domain.ServerConfig{Host: "localhost", Port: 8080, ReadTimeout: 30, WriteTimeout: 30}
	engine, _ := rules.NewEngine(nil, 5)
	return NewServer(cfg, nil, nil, b, engine, rules.NewTypologyEngine(), tadp.NewProcessor(), "test-v1", domain.ModeDetection)
}

// TestHealthEndpointBusCheck verifies the Pro tier's event bus is exercised by
// /health. Before the fix the handler held a bus reference but never pinged
// it, so a terminal NATS CLOSED state left event processing permanently dead
// while /health reported "healthy" and orchestrators never restarted the
// service.
func TestHealthEndpointBusCheck(t *testing.T) {
	t.Run("HealthyWhenBusConnected", func(t *testing.T) {
		b := bus.NewChannelBus(10)
		defer func() { _ = b.Close() }()
		server := createTestServerWithBus(b)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["status"] != "healthy" {
			t.Fatalf("expected healthy, got %v", resp["status"])
		}
	})

	t.Run("DegradedWhenBusPingFails", func(t *testing.T) {
		server := createTestServerWithBus(&fakeBus{pingErr: errors.New("NATS not connected")})

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 even when degraded, got %d: %s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["status"] != "degraded" {
			t.Fatalf("expected degraded when bus ping fails, got %v", resp["status"])
		}
	})

	t.Run("NilBusSkippedLikeCommunityTier", func(t *testing.T) {
		server := createTestServerWithBus(nil)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["status"] != "healthy" {
			t.Fatalf("expected healthy with nil bus (community tier), got %v", resp["status"])
		}
	})
}
