package bus

import (
	"fmt"

	"github.com/opensource-finance/osprey/internal/domain"
)

// New creates a new event bus based on configuration.
// For Community tier: returns ChannelBus.
// For Pro tier: returns NATSBus.
func New(cfg domain.EventBusConfig) (domain.EventBus, error) {
	switch cfg.Type {
	case "channel":
		return NewChannelBus(cfg.ChannelBufferSize), nil

	case "nats":
		return NewNATSBus(cfg)

	default:
		return nil, fmt.Errorf("unsupported event bus type: %s", cfg.Type)
	}
}
