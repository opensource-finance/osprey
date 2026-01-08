package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/opensource-finance/osprey/internal/domain"
)

// NATSBus implements EventBus using NATS.
// Used as the Pro tier event bus with resilience.
type NATSBus struct {
	mu            sync.RWMutex
	conn          *nats.Conn
	subscriptions map[string]*natsSubscription
	config        domain.EventBusConfig
}

type natsSubscription struct {
	id       string
	tenantID string
	topic    string
	sub      *nats.Subscription
}

// NewNATSBus creates a new NATS-based event bus with resilience.
func NewNATSBus(cfg domain.EventBusConfig) (*NATSBus, error) {
	if cfg.NATSUrl == "" {
		cfg.NATSUrl = nats.DefaultURL
	}
	if cfg.NATSMaxReconnects == 0 {
		cfg.NATSMaxReconnects = 10
	}
	if cfg.NATSReconnectWait == 0 {
		cfg.NATSReconnectWait = 5
	}

	// Configure NATS connection with resilience
	opts := []nats.Option{
		nats.MaxReconnects(cfg.NATSMaxReconnects),
		nats.ReconnectWait(time.Duration(cfg.NATSReconnectWait) * time.Second),
		nats.ReconnectBufSize(8 * 1024 * 1024), // 8MB buffer during reconnect
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			slog.Warn("NATS disconnected",
				"error", err,
				"will_reconnect", !nc.IsClosed(),
			)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected",
				"url", nc.ConnectedUrl(),
			)
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			slog.Info("NATS connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			slog.Error("NATS error",
				"error", err,
				"subject", sub.Subject,
			)
		}),
	}

	if cfg.NATSToken != "" {
		opts = append(opts, nats.Token(cfg.NATSToken))
	}

	// Connect with retry
	var conn *nats.Conn
	var err error
	for i := 0; i < cfg.NATSMaxReconnects; i++ {
		conn, err = nats.Connect(cfg.NATSUrl, opts...)
		if err == nil {
			break
		}
		slog.Warn("NATS connection attempt failed",
			"attempt", i+1,
			"max_attempts", cfg.NATSMaxReconnects,
			"error", err,
		)
		time.Sleep(time.Duration(cfg.NATSReconnectWait) * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS after %d attempts: %w", cfg.NATSMaxReconnects, err)
	}

	slog.Info("NATS connected",
		"url", conn.ConnectedUrl(),
		"server_id", conn.ConnectedServerId(),
	)

	return &NATSBus{
		conn:          conn,
		subscriptions: make(map[string]*natsSubscription),
		config:        cfg,
	}, nil
}

// Publish sends a message to a NATS subject.
func (b *NATSBus) Publish(ctx context.Context, tenantID string, topic string, payload []byte) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}

	// Create message envelope
	msg := &domain.Message{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Topic:     topic,
		Payload:   payload,
		Metadata:  make(map[string]string),
		Timestamp: time.Now().UnixNano(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	subject := b.makeSubject(tenantID, topic)
	return b.conn.Publish(subject, data)
}

// Subscribe registers a handler for a NATS subject.
func (b *NATSBus) Subscribe(ctx context.Context, tenantID string, topic string, handler domain.MessageHandler) (domain.Subscription, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	subject := b.makeSubject(tenantID, topic)

	// Create NATS subscription
	natsSub, err := b.conn.Subscribe(subject, func(m *nats.Msg) {
		var msg domain.Message
		if err := json.Unmarshal(m.Data, &msg); err != nil {
			slog.Error("failed to unmarshal NATS message",
				"subject", m.Subject,
				"error", err,
			)
			return
		}

		if err := handler(ctx, &msg); err != nil {
			slog.Error("handler error",
				"subject", m.Subject,
				"message_id", msg.ID,
				"error", err,
			)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	sub := &natsSubscription{
		id:       uuid.New().String(),
		tenantID: tenantID,
		topic:    topic,
		sub:      natsSub,
	}

	b.mu.Lock()
	b.subscriptions[sub.id] = sub
	b.mu.Unlock()

	return sub, nil
}

// Request implements request-reply pattern using NATS.
func (b *NATSBus) Request(ctx context.Context, tenantID string, topic string, payload []byte) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	// Create message envelope
	msg := &domain.Message{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Topic:     topic,
		Payload:   payload,
		Metadata:  make(map[string]string),
		Timestamp: time.Now().UnixNano(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	subject := b.makeSubject(tenantID, topic)

	// Get timeout from context or use default
	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	reply, err := b.conn.Request(subject, data, timeout)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Unmarshal reply
	var replyMsg domain.Message
	if err := json.Unmarshal(reply.Data, &replyMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reply: %w", err)
	}

	return replyMsg.Payload, nil
}

// Ping checks NATS connectivity.
func (b *NATSBus) Ping(ctx context.Context) error {
	if !b.conn.IsConnected() {
		return fmt.Errorf("NATS not connected")
	}
	return b.conn.FlushWithContext(ctx)
}

// Close closes the NATS connection.
func (b *NATSBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Unsubscribe all
	for _, sub := range b.subscriptions {
		_ = sub.sub.Unsubscribe()
	}
	b.subscriptions = make(map[string]*natsSubscription)

	// Close connection
	b.conn.Close()
	return nil
}

// makeSubject creates a NATS subject with tenant prefix.
func (b *NATSBus) makeSubject(tenantID, topic string) string {
	return fmt.Sprintf("osprey.%s.%s", tenantID, topic)
}

// Stats returns NATS connection statistics.
func (b *NATSBus) Stats() nats.Statistics {
	return b.conn.Stats()
}

// Unsubscribe removes the subscription.
func (s *natsSubscription) Unsubscribe() error {
	return s.sub.Unsubscribe()
}

// Topic returns the subscribed topic.
func (s *natsSubscription) Topic() string {
	return s.topic
}
