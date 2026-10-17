package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// New creates a new cache based on configuration.
// For Community tier: returns LRU cache.
// For Pro tier with two-phase: returns TwoPhaseCache wrapping LRU + Redis.
// For Pro tier without two-phase: returns Redis cache.
func New(cfg domain.CacheConfig) (domain.Cache, error) {
	switch cfg.Type {
	case "memory":
		return NewLRUCache(cfg.LocalMaxSize), nil

	case "redis":
		if cfg.EnableTwoPhase {
			return NewTwoPhaseCache(cfg)
		}
		return NewRedisCache(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)

	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cfg.Type)
	}
}

// TwoPhaseCache implements the two-phase caching strategy.
// L1: Local LRU cache for fast reads
// L2: Redis for distributed caching and persistence
type TwoPhaseCache struct {
	local  *LRUCache
	remote *RedisCache
	l1TTL  time.Duration
}

// NewTwoPhaseCache creates a two-phase cache with LRU + Redis.
func NewTwoPhaseCache(cfg domain.CacheConfig) (*TwoPhaseCache, error) {
	local := NewLRUCache(cfg.LocalMaxSize)

	remote, err := NewRedisCache(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis cache: %w", err)
	}

	l1TTL := cfg.LocalTTL
	if l1TTL == 0 {
		l1TTL = 5 * time.Minute
	}

	return &TwoPhaseCache{
		local:  local,
		remote: remote,
		l1TTL:  l1TTL,
	}, nil
}

// Get retrieves from L1 first, then L2. Populates L1 on L2 hit.
func (c *TwoPhaseCache) Get(ctx context.Context, tenantID string, key string) ([]byte, error) {
	// Check L1 first
	val, err := c.local.Get(ctx, tenantID, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		return val, nil
	}

	// Check L2
	val, err = c.remote.Get(ctx, tenantID, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		// Populate L1 for future reads
		_ = c.local.Set(ctx, tenantID, key, val, c.l1TTL)
	}

	return val, nil
}

// Set writes to both L1 and L2.
func (c *TwoPhaseCache) Set(ctx context.Context, tenantID string, key string, value []byte, ttl time.Duration) error {
	// Write to L1 with shorter TTL
	l1TTL := min(ttl, c.l1TTL)
	if err := c.local.Set(ctx, tenantID, key, value, l1TTL); err != nil {
		return err
	}

	// Write to L2 with full TTL
	return c.remote.Set(ctx, tenantID, key, value, ttl)
}

// Delete removes from both L1 and L2.
func (c *TwoPhaseCache) Delete(ctx context.Context, tenantID string, key string) error {
	if err := c.local.Delete(ctx, tenantID, key); err != nil {
		return err
	}
	return c.remote.Delete(ctx, tenantID, key)
}

// IncrementCounter uses Redis for distributed atomic counters.
// L1 is not used for counters to ensure accuracy across nodes.
func (c *TwoPhaseCache) IncrementCounter(ctx context.Context, tenantID string, key string, window time.Duration) (int64, error) {
	return c.remote.IncrementCounter(ctx, tenantID, key, window)
}

// Ping checks both L1 and L2 health.
func (c *TwoPhaseCache) Ping(ctx context.Context) error {
	if err := c.local.Ping(ctx); err != nil {
		return fmt.Errorf("L1 ping failed: %w", err)
	}
	if err := c.remote.Ping(ctx); err != nil {
		return fmt.Errorf("L2 ping failed: %w", err)
	}
	return nil
}

// Close closes both L1 and L2.
func (c *TwoPhaseCache) Close() error {
	_ = c.local.Close()
	return c.remote.Close()
}

// Stats returns L1 cache statistics.
func (c *TwoPhaseCache) Stats() (size int, capacity int) {
	return c.local.Stats()
}
