package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/redis/go-redis/v9"
)

// RedisCache implements Cache using Redis.
// Used as the Pro tier cache and as L2 in two-phase caching.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache.
func NewRedisCache(addr, password string, db int) (*RedisCache, error) {
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// Get retrieves a value from Redis.
func (c *RedisCache) Get(ctx context.Context, tenantID string, key string) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	fullKey := c.makeKey(tenantID, key)
	val, err := c.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Set stores a value in Redis with TTL.
func (c *RedisCache) Set(ctx context.Context, tenantID string, key string, value []byte, ttl time.Duration) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}

	fullKey := c.makeKey(tenantID, key)
	return c.client.Set(ctx, fullKey, value, ttl).Err()
}

// Delete removes a value from Redis.
func (c *RedisCache) Delete(ctx context.Context, tenantID string, key string) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}

	fullKey := c.makeKey(tenantID, key)
	return c.client.Del(ctx, fullKey).Err()
}

// GetTransaction retrieves cached transaction data.
func (c *RedisCache) GetTransaction(ctx context.Context, tenantID string, txID string) (*domain.DataCache, error) {
	data, err := c.Get(ctx, tenantID, "tx:"+txID)
	if err != nil || data == nil {
		return nil, err
	}

	var dc domain.DataCache
	if err := json.Unmarshal(data, &dc); err != nil {
		return nil, err
	}
	return &dc, nil
}

// SetTransaction caches transaction data.
func (c *RedisCache) SetTransaction(ctx context.Context, tenantID string, txID string, data *domain.DataCache, ttl time.Duration) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.Set(ctx, tenantID, "tx:"+txID, bytes, ttl)
}

// IncrementCounter atomically increments a counter using Redis INCR with EXPIRE.
func (c *RedisCache) IncrementCounter(ctx context.Context, tenantID string, key string, window time.Duration) (int64, error) {
	if tenantID == "" {
		return 0, fmt.Errorf("tenantID is required")
	}

	fullKey := c.makeKey(tenantID, "counter:"+key)

	// Use Lua script for atomic increment with TTL
	script := redis.NewScript(`
		local current = redis.call('INCR', KEYS[1])
		if current == 1 then
			redis.call('PEXPIRE', KEYS[1], ARGV[1])
		end
		return current
	`)

	result, err := script.Run(ctx, c.client, []string{fullKey}, window.Milliseconds()).Int64()
	if err != nil {
		return 0, err
	}

	return result, nil
}

// Ping checks Redis connectivity.
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) makeKey(tenantID, key string) string {
	return "osprey:" + tenantID + ":" + key
}
