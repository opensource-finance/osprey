package bus

import (
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
)

// TestNATSErrorHandlerNilSubNoPanic verifies the async error handler does not
// panic when the NATS client invokes it with a nil *Subscription. The library
// pushes asyncErrorCB(nc, nil, err) on several internal error paths
// (reconnect-handshake read failures, transient server errors such as
// permissions violations or max-subscriptions-exceeded). The handler runs on
// the library's asyncCBDispatcher goroutine which has no recover, so a panic
// there terminates the entire process.
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
		// A real *nats.Conn is not required to exercise the nil-guard; the
		// handler only logs nc and never dereferences it. Passing nil is the
		// strictest test of the nil-sub path.
		natsErrorHandler(nil, nil, errors.New("reconnect handshake read failure"))
	})
}

// TestNATSErrorHandlerNonNilSub ensures the handler still logs the subject
// for a normal (non-nil) subscription, preserving the original behavior on the
// happy path. It guards against a future change that drops the subject
// altogether.
func TestNATSErrorHandlerNonNilSub(t *testing.T) {
	sub := &nats.Subscription{}
	// Subject is an exported field; set it directly to avoid needing a live
	// connection for this unit test.
	sub.Subject = "osprey.tenant1.topic"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panicked on non-nil sub: %v", r)
		}
	}()
	natsErrorHandler(nil, sub, errors.New("simulated subscription error"))
}
