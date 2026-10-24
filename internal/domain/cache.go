package domain

import (
	"context"
	"time"
)

// Cache defines the interface for caching operations.
// Supports two-phase caching: local LRU (Community) + Redis (Pro).
// All methods require tenantID for strict multi-tenancy isolation.
type Cache interface {
	// Get retrieves a value from cache.
	// Returns nil, nil if key not found.
	Get(ctx context.Context, tenantID string, key string) ([]byte, error)

	// Set stores a value in cache with expiration.
	Set(ctx context.Context, tenantID string, key string, value []byte, ttl time.Duration) error

	// Delete removes a value from cache.
	Delete(ctx context.Context, tenantID string, key string) error

	// IncrementCounter atomically increments a counter and returns new value.
	// Used for velocity checks (e.g., transaction count in time window).
	IncrementCounter(ctx context.Context, tenantID string, key string, window time.Duration) (int64, error)

	// Health check
	Ping(ctx context.Context) error

	// Lifecycle
	Close() error
}

// CacheConfig holds configuration for cache initialization.
type CacheConfig struct {
	// Type is the cache type: "memory" or "redis"
	Type string

	// Local LRU cache settings (Community tier)
	LocalMaxSize int
	LocalTTL     time.Duration

	// Redis settings (Pro tier)
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Two-phase settings
	EnableTwoPhase bool // If true, check local first, then Redis
}
