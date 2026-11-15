// Package bus provides event bus implementations for Osprey.
package bus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opensource-finance/osprey/internal/domain"
)

// ChannelBus implements EventBus using Go channels.
// Used as the Community tier event bus.
type ChannelBus struct {
	mu            sync.RWMutex
	bufferSize    int
	subscriptions map[string][]*channelSubscription
	closed        bool
}

type channelSubscription struct {
	id       string
	tenantID string
	topic    string
	handler  domain.MessageHandler
	msgCh    chan *domain.Message
	ctx      context.Context
	cancel   context.CancelFunc
	bus      *ChannelBus
}

// NewChannelBus creates a new channel-based event bus.
func NewChannelBus(bufferSize int) *ChannelBus {
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	return &ChannelBus{
		bufferSize:    bufferSize,
		subscriptions: make(map[string][]*channelSubscription),
	}
}

// Publish sends a message to a topic.
func (b *ChannelBus) Publish(ctx context.Context, tenantID string, topic string, payload []byte) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return fmt.Errorf("bus is closed")
	}

	// Create message
	msg := &domain.Message{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Topic:     topic,
		Payload:   payload,
		Metadata:  make(map[string]string),
		Timestamp: time.Now().UnixNano(),
	}

	// Get the exact-tenant subscriptions for this topic.
	subs := b.subscriptions[b.makeKey(tenantID, topic)]
	// Also get the wildcard subscriptions (tenant domain.GlobalTenantID). A
	// wildcard subscriber receives every publish on this topic regardless of
	// tenant, so the global worker can process all tenants. The published
	// message keeps the real tenantID, so the handler can attribute it.
	// Skip the wildcard bucket when the publisher used the wildcard tenant
	// itself, because then the exact-tenant bucket is already the wildcard
	// bucket and delivering both would duplicate the message.
	var wildcardSubs []*channelSubscription
	if tenantID != domain.GlobalTenantID {
		wildcardSubs = b.subscriptions[b.makeKey(domain.GlobalTenantID, topic)]
	}
	b.mu.RUnlock()

	// Send to all matching subscribers (non-blocking)
	for _, sub := range subs {
		select {
		case sub.msgCh <- msg:
		default:
			// Channel full, skip this message for this subscriber
		}
	}
	for _, sub := range wildcardSubs {
		select {
		case sub.msgCh <- msg:
		default:
			// Channel full, skip this message for this subscriber
		}
	}

	return nil
}

// Subscribe registers a handler for a topic.
//
// tenantID scopes the subscription. A regular tenantID receives only the
// messages published for that tenant. The special value domain.GlobalTenantID
// ("*") registers a wildcard subscription: Publish delivers to it for every
// tenant on the topic, and the delivered message carries the real publishing
// tenant, so the handler can attribute the work.
func (b *ChannelBus) Subscribe(ctx context.Context, tenantID string, topic string, handler domain.MessageHandler) (domain.Subscription, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, fmt.Errorf("bus is closed")
	}

	subCtx, cancel := context.WithCancel(ctx)

	sub := &channelSubscription{
		id:       uuid.New().String(),
		tenantID: tenantID,
		topic:    topic,
		handler:  handler,
		msgCh:    make(chan *domain.Message, b.bufferSize),
		ctx:      subCtx,
		cancel:   cancel,
		bus:      b,
	}

	// Start message handler goroutine
	go b.handleMessages(sub)

	key := b.makeKey(tenantID, topic)
	b.subscriptions[key] = append(b.subscriptions[key], sub)

	return sub, nil
}

// handleMessages processes messages for a subscription.
func (b *ChannelBus) handleMessages(sub *channelSubscription) {
	for {
		select {
		case <-sub.ctx.Done():
			return
		case msg := <-sub.msgCh:
			// select picks at random when both cases are ready, so check the
			// context again: a cancelled subscription never runs its handler.
			if msg != nil && sub.ctx.Err() == nil {
				_ = sub.handler(sub.ctx, msg)
			}
		}
	}
}

// Ping checks bus health.
func (b *ChannelBus) Ping(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return fmt.Errorf("bus is closed")
	}
	return nil
}

// Close closes the event bus.
func (b *ChannelBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	// Cancel all subscriptions.
	// ponytail: cancel only, never close(msgCh) — Publish sends without the lock
	// (select+default), so closing here races into a send-on-closed-channel panic.
	// handleMessages exits on ctx.Done(); nothing ranges over msgCh.
	for _, subs := range b.subscriptions {
		for _, sub := range subs {
			sub.cancel()
		}
	}

	b.subscriptions = make(map[string][]*channelSubscription)
	return nil
}

func (b *ChannelBus) makeKey(tenantID, topic string) string {
	return tenantID + ":" + topic
}

// Unsubscribe stops receiving messages. It removes the subscription from the
// bus first, so a later Publish cannot reach it, then cancels it, so a message
// already queued is dropped and the handler does not run again.
func (s *channelSubscription) Unsubscribe() error {
	if s.bus != nil {
		s.bus.removeSubscription(s)
	}
	s.cancel()
	return nil
}

// removeSubscription deletes sub from its topic's list. Publish reads the list
// under the read lock and then sends without it, so the list is replaced, not
// changed in place.
func (b *ChannelBus) removeSubscription(sub *channelSubscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := b.makeKey(sub.tenantID, sub.topic)
	subs := b.subscriptions[key]
	kept := make([]*channelSubscription, 0, len(subs))
	for _, s := range subs {
		if s != sub {
			kept = append(kept, s)
		}
	}
	if len(kept) == 0 {
		delete(b.subscriptions, key)
		return
	}
	b.subscriptions[key] = kept
}

// Topic returns the subscribed topic.
func (s *channelSubscription) Topic() string {
	return s.topic
}
