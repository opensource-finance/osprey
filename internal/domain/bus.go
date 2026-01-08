package domain

import (
	"context"
)

// EventBus defines the interface for event-driven communication.
// Supports Go channels (Community) or NATS (Pro).
// All methods require tenantID for strict multi-tenancy isolation.
type EventBus interface {
	// Publish sends a message to a topic.
	Publish(ctx context.Context, tenantID string, topic string, payload []byte) error

	// Subscribe registers a handler for a topic.
	// Returns a subscription that can be used to unsubscribe.
	Subscribe(ctx context.Context, tenantID string, topic string, handler MessageHandler) (Subscription, error)

	// Request sends a message and waits for a response (request-reply pattern).
	Request(ctx context.Context, tenantID string, topic string, payload []byte) ([]byte, error)

	// Health check
	Ping(ctx context.Context) error

	// Lifecycle
	Close() error
}

// MessageHandler processes incoming messages.
type MessageHandler func(ctx context.Context, msg *Message) error

// Message represents an event message.
type Message struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenantId"`
	Topic     string            `json:"topic"`
	Payload   []byte            `json:"payload"`
	Metadata  map[string]string `json:"metadata"`
	Timestamp int64             `json:"timestamp"`
}

// Subscription represents an active subscription.
type Subscription interface {
	// Unsubscribe stops receiving messages.
	Unsubscribe() error

	// Topic returns the subscribed topic.
	Topic() string
}

// EventBusConfig holds configuration for event bus initialization.
type EventBusConfig struct {
	// Type is the bus type: "channel" or "nats"
	Type string

	// Channel settings (Community tier)
	ChannelBufferSize int

	// NATS settings (Pro tier)
	NATSUrl           string
	NATSToken         string
	NATSMaxReconnects int
	NATSReconnectWait int // seconds
}

// Standard topic names for the evaluation pipeline.
const (
	TopicTransactionIngested = "osprey.transaction.ingested"
	TopicRuleEvaluate        = "osprey.rule.evaluate"
	TopicRuleResult          = "osprey.rule.result"
	TopicTypologyEvaluate    = "osprey.typology.evaluate"
	TopicTypologyResult      = "osprey.typology.result"
	TopicDecision            = "osprey.decision"
	TopicAlert               = "osprey.alert"
)
