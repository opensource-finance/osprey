package bus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestChannelBus(t *testing.T) {
	bus := NewChannelBus(100)
	defer bus.Close()

	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("PublishAndSubscribe", func(t *testing.T) {
		var received atomic.Bool
		var receivedMsg *domain.Message

		var wg sync.WaitGroup
		wg.Add(1)

		_, err := bus.Subscribe(ctx, tenantID, "test.topic", func(ctx context.Context, msg *domain.Message) error {
			receivedMsg = msg
			received.Store(true)
			wg.Done()
			return nil
		})
		if err != nil {
			t.Fatalf("subscribe failed: %v", err)
		}

		// Allow subscription to be active
		time.Sleep(10 * time.Millisecond)

		err = bus.Publish(ctx, tenantID, "test.topic", []byte("hello"))
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}

		// Wait for message
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for message")
		}

		if !received.Load() {
			t.Error("message not received")
		}

		if string(receivedMsg.Payload) != "hello" {
			t.Errorf("expected payload 'hello', got '%s'", string(receivedMsg.Payload))
		}
		if receivedMsg.TenantID != tenantID {
			t.Errorf("expected tenantID '%s', got '%s'", tenantID, receivedMsg.TenantID)
		}
	})

	t.Run("TenantIsolation", func(t *testing.T) {
		tenant1 := "tenant-001"
		tenant2 := "tenant-002"

		var received1 atomic.Int32
		var received2 atomic.Int32

		bus.Subscribe(ctx, tenant1, "isolation.topic", func(ctx context.Context, msg *domain.Message) error {
			received1.Add(1)
			return nil
		})

		bus.Subscribe(ctx, tenant2, "isolation.topic", func(ctx context.Context, msg *domain.Message) error {
			received2.Add(1)
			return nil
		})

		time.Sleep(10 * time.Millisecond)

		// Publish to tenant1
		bus.Publish(ctx, tenant1, "isolation.topic", []byte("msg1"))
		time.Sleep(50 * time.Millisecond)

		if received1.Load() != 1 {
			t.Errorf("tenant1 should receive 1 message, got %d", received1.Load())
		}
		if received2.Load() != 0 {
			t.Errorf("tenant2 should receive 0 messages, got %d", received2.Load())
		}
	})

	t.Run("RequiresTenantID", func(t *testing.T) {
		err := bus.Publish(ctx, "", "topic", []byte("data"))
		if err == nil {
			t.Error("expected error for empty tenantID")
		}

		_, err = bus.Subscribe(ctx, "", "topic", func(ctx context.Context, msg *domain.Message) error {
			return nil
		})
		if err == nil {
			t.Error("expected error for empty tenantID")
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		var count atomic.Int32

		sub, _ := bus.Subscribe(ctx, tenantID, "unsub.topic", func(ctx context.Context, msg *domain.Message) error {
			count.Add(1)
			return nil
		})

		time.Sleep(10 * time.Millisecond)

		bus.Publish(ctx, tenantID, "unsub.topic", []byte("msg1"))
		time.Sleep(50 * time.Millisecond)

		if count.Load() != 1 {
			t.Errorf("expected 1 message before unsubscribe, got %d", count.Load())
		}

		sub.Unsubscribe()
		time.Sleep(10 * time.Millisecond)

		bus.Publish(ctx, tenantID, "unsub.topic", []byte("msg2"))
		time.Sleep(50 * time.Millisecond)

		// Should still be 1 after unsubscribe
		if count.Load() != 1 {
			t.Errorf("expected 1 message after unsubscribe, got %d", count.Load())
		}
	})

	t.Run("MultipleSubscribers", func(t *testing.T) {
		var count1, count2 atomic.Int32

		bus.Subscribe(ctx, tenantID, "multi.topic", func(ctx context.Context, msg *domain.Message) error {
			count1.Add(1)
			return nil
		})

		bus.Subscribe(ctx, tenantID, "multi.topic", func(ctx context.Context, msg *domain.Message) error {
			count2.Add(1)
			return nil
		})

		time.Sleep(10 * time.Millisecond)

		bus.Publish(ctx, tenantID, "multi.topic", []byte("broadcast"))
		time.Sleep(50 * time.Millisecond)

		if count1.Load() != 1 || count2.Load() != 1 {
			t.Errorf("expected both subscribers to receive, got %d and %d", count1.Load(), count2.Load())
		}
	})

	t.Run("Ping", func(t *testing.T) {
		if err := bus.Ping(ctx); err != nil {
			t.Errorf("ping failed: %v", err)
		}
	})

	t.Run("SubscriptionTopic", func(t *testing.T) {
		sub, _ := bus.Subscribe(ctx, tenantID, "my.topic", func(ctx context.Context, msg *domain.Message) error {
			return nil
		})

		if sub.Topic() != "my.topic" {
			t.Errorf("expected topic 'my.topic', got '%s'", sub.Topic())
		}
	})
}

func TestChannelBusClose(t *testing.T) {
	bus := NewChannelBus(100)

	ctx := context.Background()
	tenantID := "tenant-001"

	bus.Subscribe(ctx, tenantID, "close.topic", func(ctx context.Context, msg *domain.Message) error {
		return nil
	})

	if err := bus.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}

	// Operations should fail after close
	if err := bus.Publish(ctx, tenantID, "close.topic", []byte("data")); err == nil {
		t.Error("expected error after close")
	}

	if err := bus.Ping(ctx); err == nil {
		t.Error("expected ping error after close")
	}
}

func TestNewBus(t *testing.T) {
	t.Run("ChannelType", func(t *testing.T) {
		cfg := domain.EventBusConfig{
			Type:              "channel",
			ChannelBufferSize: 50,
		}

		bus, err := New(cfg)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		defer bus.Close()

		_, ok := bus.(*ChannelBus)
		if !ok {
			t.Error("expected ChannelBus for channel type")
		}
	})

	t.Run("UnsupportedType", func(t *testing.T) {
		cfg := domain.EventBusConfig{
			Type: "kafka",
		}

		_, err := New(cfg)
		if err == nil {
			t.Error("expected error for unsupported type")
		}
	})
}

func TestChannelBusHighLoad(t *testing.T) {
	bus := NewChannelBus(1000)
	defer bus.Close()

	ctx := context.Background()
	tenantID := "tenant-load"

	var received atomic.Int32
	const messageCount = 100

	var wg sync.WaitGroup
	wg.Add(messageCount)

	bus.Subscribe(ctx, tenantID, "load.topic", func(ctx context.Context, msg *domain.Message) error {
		received.Add(1)
		wg.Done()
		return nil
	})

	time.Sleep(10 * time.Millisecond)

	// Publish many messages
	for i := 0; i < messageCount; i++ {
		bus.Publish(ctx, tenantID, "load.topic", []byte("msg"))
	}

	// Wait for all messages
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if received.Load() != messageCount {
			t.Errorf("expected %d messages, got %d", messageCount, received.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout: received %d/%d messages", received.Load(), messageCount)
	}
}
