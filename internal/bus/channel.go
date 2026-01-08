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

	// Get subscriptions for this topic
	subs := b.subscriptions[b.makeKey(tenantID, topic)]
	b.mu.RUnlock()

	// Send to all matching subscribers (non-blocking)
	for _, sub := range subs {
		select {
		case sub.msgCh <- msg:
		default:
			// Channel full, skip this message for this subscriber
		}
	}

	return nil
}

// Subscribe registers a handler for a topic.
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
			if msg != nil {
				_ = sub.handler(sub.ctx, msg)
			}
		}
	}
}

// Request implements request-reply pattern using channels.
func (b *ChannelBus) Request(ctx context.Context, tenantID string, topic string, payload []byte) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	// Create reply channel
	replyCh := make(chan []byte, 1)
	replyTopic := topic + ".reply." + uuid.New().String()

	// Subscribe to reply
	sub, err := b.Subscribe(ctx, tenantID, replyTopic, func(ctx context.Context, msg *domain.Message) error {
		select {
		case replyCh <- msg.Payload:
		default:
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	defer sub.Unsubscribe()

	// Publish request
	if err := b.Publish(ctx, tenantID, topic, payload); err != nil {
		return nil, err
	}

	// Wait for reply with timeout
	select {
	case reply := <-replyCh:
		return reply, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
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

	// Cancel all subscriptions
	for _, subs := range b.subscriptions {
		for _, sub := range subs {
			sub.cancel()
			close(sub.msgCh)
		}
	}

	b.subscriptions = make(map[string][]*channelSubscription)
	return nil
}

func (b *ChannelBus) makeKey(tenantID, topic string) string {
	return tenantID + ":" + topic
}

// Unsubscribe stops receiving messages.
func (s *channelSubscription) Unsubscribe() error {
	s.cancel()
	return nil
}

// Topic returns the subscribed topic.
func (s *channelSubscription) Topic() string {
	return s.topic
}
