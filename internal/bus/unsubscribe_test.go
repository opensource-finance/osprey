package bus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// After Unsubscribe returns, the handler must never run again, and the
// subscription must leave the bus.
func TestChannelBusUnsubscribeStopsDelivery(t *testing.T) {
	b := NewChannelBus(16)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	var late atomic.Int64
	for round := 0; round < 300; round++ {
		var unsubscribed atomic.Bool
		sub, err := b.Subscribe(ctx, "t1", "topic", func(context.Context, *domain.Message) error {
			if unsubscribed.Load() {
				late.Add(1)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := sub.Unsubscribe(); err != nil {
			t.Fatal(err)
		}
		unsubscribed.Store(true)
		if err := b.Publish(ctx, "t1", "topic", []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if n := late.Load(); n > 0 {
		t.Errorf("handler ran %d times after Unsubscribe", n)
	}
	b.mu.RLock()
	left := len(b.subscriptions[b.makeKey("t1", "topic")])
	b.mu.RUnlock()
	if left != 0 {
		t.Errorf("%d subscriptions still registered after Unsubscribe", left)
	}
}
