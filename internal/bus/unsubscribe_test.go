package bus

import (
	"context"
	"fmt"
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
	for range 300 {
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

// A message still queued when Unsubscribe returns is dropped: the handler does
// not run for it. Each subscription blocks its handler on the first message,
// so the later messages wait in the channel while Unsubscribe runs.
func TestChannelBusUnsubscribeDropsQueuedMessages(t *testing.T) {
	b := NewChannelBus(16)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	const subs = 20
	release := make(chan struct{})
	calls := make([]atomic.Int64, subs)
	for i := range subs {
		topic := fmt.Sprintf("queued-%d", i)
		started := make(chan struct{})
		sub, err := b.Subscribe(ctx, "t1", topic, func(context.Context, *domain.Message) error {
			if calls[i].Add(1) == 1 {
				close(started)
				<-release
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for range 5 {
			if err := b.Publish(ctx, "t1", topic, []byte("x")); err != nil {
				t.Fatal(err)
			}
		}
		<-started
		if err := sub.Unsubscribe(); err != nil {
			t.Fatal(err)
		}
	}
	close(release)
	time.Sleep(100 * time.Millisecond)
	for i := range subs {
		if n := calls[i].Load(); n != 1 {
			t.Errorf("subscription %d: handler ran %d times, want 1 (queued messages ran after Unsubscribe)", i, n)
		}
	}
}

// A subscription whose context ends without Unsubscribe also leaves the bus.
func TestChannelBusCancelledContextLeavesBus(t *testing.T) {
	b := NewChannelBus(16)
	defer func() { _ = b.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	if _, err := b.Subscribe(ctx, "t1", "topic", func(context.Context, *domain.Message) error { return nil }); err != nil {
		t.Fatal(err)
	}
	cancel()

	deadline := time.Now().Add(time.Second)
	for {
		b.mu.RLock()
		left := len(b.subscriptions[b.makeKey("t1", "topic")])
		b.mu.RUnlock()
		if left == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d subscriptions still registered after their context ended", left)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
