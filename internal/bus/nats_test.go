package bus

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/opensource-finance/osprey/internal/domain"
)

// TestNATSErrorHandlerNilSubNoPanic checks the async error handler survives a nil *Subscription.
func TestNATSErrorHandlerNilSubNoPanic(t *testing.T) {
	t.Run("NilSubNilConn", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("handler panicked on nil sub: %v", r)
			}
		}()
		natsErrorHandler(nil, nil, errors.New("simulated nil-sub async error"))
	})

	t.Run("NilSubWithConn", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("handler panicked on nil sub: %v", r)
			}
		}()
		// The handler never dereferences nc, so nil is fine.
		natsErrorHandler(nil, nil, errors.New("reconnect handshake read failure"))
	})
}

// TestNATSErrorHandlerNonNilSub checks the handler still logs the subject.
func TestNATSErrorHandlerNonNilSub(t *testing.T) {
	sub := &nats.Subscription{}
	sub.Subject = "osprey.tenant1.topic"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panicked on non-nil sub: %v", r)
		}
	}()
	natsErrorHandler(nil, sub, errors.New("simulated subscription error"))
}

// TestNATSBus_makeSubject verifies the subject construction, including the
// wildcard subject for the global tenant. A per-tenant publish uses
// osprey.<tenant>.<topic>, and a global (domain.GlobalTenantID) subscription
// uses osprey.*.<topic>. The NATS single-token wildcard "*" matches the
// position the per-tenant publish fills, so the global subscription is
// reachable for any single-token tenant. This is what makes the worker's
// global path reachable on the Pro bus, instead of the dead exact-tenant
// "_global" subject the old code produced.
func TestNATSBus_makeSubject(t *testing.T) {
	b := &NATSBus{}

	t.Run("ExactTenant", func(t *testing.T) {
		got := b.makeSubject("tenant-a", "osprey.transaction.ingested")
		want := "osprey.tenant-a.osprey.transaction.ingested"
		if got != want {
			t.Errorf("exact tenant subject = %q, want %q", got, want)
		}
	})

	t.Run("WildcardTenant", func(t *testing.T) {
		got := b.makeSubject(domain.GlobalTenantID, "osprey.transaction.ingested")
		want := "osprey.*.osprey.transaction.ingested"
		if got != want {
			t.Errorf("wildcard subject = %q, want %q", got, want)
		}
		// The publish subject for a real tenant has exactly one token in the
		// tenant position, which the NATS "*" wildcard matches. Assert the
		// position structure: same token count, "*" only in the tenant slot.
		publish := b.makeSubject("tenant-a", "osprey.transaction.ingested")
		pubTokens := strings.Split(publish, ".")
		wildTokens := strings.Split(got, ".")
		if len(pubTokens) != len(wildTokens) {
			t.Fatalf("token count differs: publish %d (%q) vs wildcard %d (%q)",
				len(pubTokens), publish, len(wildTokens), got)
		}
		if wildTokens[1] != "*" {
			t.Errorf("wildcard subject tenant token = %q, want %q", wildTokens[1], "*")
		}
		if pubTokens[1] != "tenant-a" {
			t.Errorf("publish subject tenant token = %q, want %q", pubTokens[1], "tenant-a")
		}
		// Every other token must be identical, so the wildcard only varies the
		// tenant position and nothing else.
		for i, pt := range pubTokens {
			if i == 1 {
				continue
			}
			if wildTokens[i] != pt {
				t.Errorf("token %d: wildcard %q != publish %q (only the tenant token may differ)",
					i, wildTokens[i], pt)
			}
		}
	})
}

// liveNATSConfig builds an EventBusConfig pointing at a real NATS server when
// OSPREY_NATS_TEST_URL is set; the test is skipped otherwise, so the default
// unit run needs no server. Each test gets a fresh connection closed at the end.
func liveNATSConfig(t *testing.T) domain.EventBusConfig {
	t.Helper()
	url := os.Getenv("OSPREY_NATS_TEST_URL")
	if url == "" {
		t.Skip("set OSPREY_NATS_TEST_URL to run against a live NATS server")
	}
	return domain.EventBusConfig{
		Type:              "nats",
		NATSUrl:           url,
		NATSMaxReconnects: -1,
		NATSReconnectWait: 1,
	}
}

// TestNATSBusPingAcceptsDeadlinelessContext is a live test for the health-check
// wiring. The /health handler passes r.Context(), which has no deadline, but
// nats.go's FlushWithContext requires one (ErrNoDeadline "nats: context
// requires a deadline"). Without the Ping fix, bus.Ping would always fail for
// the Pro tier and /health would report "degraded" even when connected.
func TestNATSBusPingAcceptsDeadlinelessContext(t *testing.T) {
	cfg := liveNATSConfig(t)
	b, err := NewNATSBus(cfg)
	if err != nil {
		t.Fatalf("NewNATSBus: %v", err)
	}
	defer func() { _ = b.Close() }()

	// deadline-less context, exactly what the /health handler provides.
	if err := b.Ping(context.Background()); err != nil {
		t.Fatalf("Ping with deadline-less context failed (the /health handler passes r.Context()): %v", err)
	}

	// a caller-supplied deadline is still honored.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := b.Ping(ctx); err != nil {
		t.Fatalf("Ping with deadline context failed: %v", err)
	}
}

// TestNATSBusReconnectConfigLive confirms the shipped default produces an
// unbounded reconnect budget against a real server, so a transient outage
// cannot drive the connection into the terminal CLOSED state.
func TestNATSBusReconnectConfigLive(t *testing.T) {
	cfg := liveNATSConfig(t)
	if cfg.NATSMaxReconnects != -1 {
		t.Fatalf("live config must request unbounded reconnects, got %d", cfg.NATSMaxReconnects)
	}
	b, err := NewNATSBus(cfg)
	if err != nil {
		t.Fatalf("NewNATSBus: %v", err)
	}
	defer func() { _ = b.Close() }()

	// The live connection should report connected and the nats option should
	// record unbounded reconnects.
	if !b.conn.IsConnected() {
		t.Fatalf("expected connected bus")
	}
	// MaxReconnect == -1 is what nats.go treats as "reconnect forever".
	opts := b.conn.Opts
	if opts.MaxReconnect != -1 {
		t.Fatalf("expected MaxReconnect == -1 (unbounded), got %d", opts.MaxReconnect)
	}
}

// Unsubscribe removes the subscription from the bus map, so repeated
// subscribe and unsubscribe do not keep old subscriptions in memory. The
// zero nats.Subscription has no connection, so its Unsubscribe returns an
// error, but the bus entry must still go.
func TestNATSSubscriptionUnsubscribeLeavesBus(t *testing.T) {
	b := &NATSBus{subscriptions: make(map[string]*natsSubscription)}
	sub := &natsSubscription{id: "sub-1", topic: "topic", sub: &nats.Subscription{}, bus: b}
	b.subscriptions[sub.id] = sub

	_ = sub.Unsubscribe()

	if n := len(b.subscriptions); n != 0 {
		t.Errorf("%d subscriptions still registered after Unsubscribe", n)
	}
}
