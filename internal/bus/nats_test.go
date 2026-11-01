package bus

import (
	"errors"
	"strings"
	"testing"

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
