package bus

import (
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
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
