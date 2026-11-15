package bus

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/opensource-finance/osprey/internal/domain"
)

// TestResolveMaxReconnects verifies the reconnect-budget mapping that is the
// core of the outage-recovery fix: the zero-value default (and any negative
// value) resolves to -1, which nats.go treats as unbounded reconnects. A
// transient NATS outage can no longer exhaust the budget and drive the
// connection into the terminal CLOSED state. A positive value is honored as
// a bounded budget for operators who want fail-fast wiring.
func TestResolveMaxReconnects(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero default maps to unbounded", 0, -1},
		{"negative one maps to unbounded", -1, -1},
		{"large negative maps to unbounded", -42, -1},
		{"positive one honored", 1, 1},
		{"positive ten honored", 10, 10},
		{"positive hundred honored", 100, 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveMaxReconnects(tc.in); got != tc.want {
				t.Errorf("resolveMaxReconnects(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestMaxReconnectsOptionWiring proves the fix at the nats.go option level:
// the default config flows through resolveMaxReconnects into
// nats.MaxReconnects(-1), which nats.go records as unbounded reconnects. With
// the old code the option received a bounded 10, so an outage longer than the
// budget left the connection permanently CLOSED.
func TestMaxReconnectsOptionWiring(t *testing.T) {
	t.Run("default config yields unbounded reconnects", func(t *testing.T) {
		opts := nats.GetDefaultOptions()
		if err := nats.MaxReconnects(resolveMaxReconnects(0))(&opts); err != nil {
			t.Fatalf("apply MaxReconnects option: %v", err)
		}
		if opts.MaxReconnect != -1 {
			t.Fatalf("default config must produce unbounded reconnects, got MaxReconnect=%d", opts.MaxReconnect)
		}
	})

	t.Run("explicit bounded config honored", func(t *testing.T) {
		opts := nats.GetDefaultOptions()
		if err := nats.MaxReconnects(resolveMaxReconnects(5))(&opts); err != nil {
			t.Fatalf("apply MaxReconnects option: %v", err)
		}
		if opts.MaxReconnect != 5 {
			t.Fatalf("bounded config must be honored, got MaxReconnect=%d", opts.MaxReconnect)
		}
	})
}

// freePort returns a TCP port that is currently free on localhost. The caller
// never listens on it, so nats.Connect gets connection-refused immediately.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// TestNewNATSBusFailsFastWhenServerUnreachable guards the unbounded-reconnect
// change: runtime reconnects are unbounded by default, but the initial
// connection loop must stay bounded so startup fails fast (and never hangs)
// when NATS is unreachable.
func TestNewNATSBusFailsFastWhenServerUnreachable(t *testing.T) {
	port := freePort(t)
	cfg := domain.EventBusConfig{
		Type:              "nats",
		NATSUrl:           fmt.Sprintf("nats://127.0.0.1:%d", port),
		NATSMaxReconnects: 1,
		NATSReconnectWait: 1,
	}

	type result struct {
		bus *NATSBus
		err error
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		b, err := NewNATSBus(cfg)
		done <- result{b, err}
	}()

	select {
	case r := <-done:
		if r.err == nil {
			_ = r.bus.Close()
			t.Fatal("expected NewNATSBus to fail when NATS is unreachable, got nil error")
		}
		if elapsed := time.Since(start); elapsed > 15*time.Second {
			t.Fatalf("NewNATSBus took %s to fail; initial connect must fail fast", elapsed)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("NewNATSBus hung trying to reach an unreachable server; initial connect must be bounded")
	}
}
