package domain

import "testing"

// TestProConfigNATSReconnectsUnbounded is the regression test for the Pro-tier
// NATS outage bug. The shipped ProConfig default must request unbounded
// reconnects (NATSMaxReconnects <= 0, mapped to nats.MaxReconnects(-1)); the
// old shipped default of 10 exhausted the budget after ~50s and drove the
// connection into the terminal CLOSED state, permanently killing async event
// processing until the process restarted.
func TestProConfigNATSReconnectsUnbounded(t *testing.T) {
	cfg := ProConfig()
	if cfg.EventBus.Type != "nats" {
		t.Fatalf("expected nats bus type, got %q", cfg.EventBus.Type)
	}
	if cfg.EventBus.NATSMaxReconnects > 0 {
		t.Fatalf("shipped ProConfig must default to unbounded NATS reconnects (<=0), got %d — a positive budget re-introduces the terminal-CLOSED outage bug", cfg.EventBus.NATSMaxReconnects)
	}
}
