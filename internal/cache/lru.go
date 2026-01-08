// Package cache provides caching implementations for Osprey.
package cache

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// LRUCache is a thread-safe LRU cache with TTL support.
// Used as the Community tier cache and as L1 in two-phase caching.
type LRUCache struct {
	mu       sync.RWMutex
	maxSize  int
	items    map[string]*list.Element
	order    *list.List
	counters map[string]*counterEntry
}

type cacheEntry struct {
	key       string
	value     []byte
	expiresAt time.Time
}

type counterEntry struct {
	count     int64
	expiresAt time.Time
}

// NewLRUCache creates a new LRU cache with the specified max size.
func NewLRUCache(maxSize int) *LRUCache {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &LRUCache{
		maxSize:  maxSize,
		items:    make(map[string]*list.Element),
		order:    list.New(),
		counters: make(map[string]*counterEntry),
	}
}

// Get retrieves a value from cache.
func (c *LRUCache) Get(ctx context.Context, tenantID string, key string) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	fullKey := c.makeKey(tenantID, key)

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[fullKey]
	if !ok {
		return nil, nil
	}

	entry := elem.Value.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.removeElement(elem)
		return nil, nil
	}

	// Move to front (most recently used)
	c.order.MoveToFront(elem)
	return entry.value, nil
}

// Set stores a value in cache with TTL.
func (c *LRUCache) Set(ctx context.Context, tenantID string, key string, value []byte, ttl time.Duration) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}

	fullKey := c.makeKey(tenantID, key)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry
	if elem, ok := c.items[fullKey]; ok {
		c.order.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.value = value
		entry.expiresAt = time.Now().Add(ttl)
		return nil
	}

	// Add new entry
	entry := &cacheEntry{
		key:       fullKey,
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	elem := c.order.PushFront(entry)
	c.items[fullKey] = elem

	// Evict if over capacity
	for c.order.Len() > c.maxSize {
		c.removeOldest()
	}

	return nil
}

// Delete removes a value from cache.
func (c *LRUCache) Delete(ctx context.Context, tenantID string, key string) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}

	fullKey := c.makeKey(tenantID, key)

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[fullKey]; ok {
		c.removeElement(elem)
	}
	return nil
}

// GetTransaction retrieves cached transaction data.
func (c *LRUCache) GetTransaction(ctx context.Context, tenantID string, txID string) (*domain.DataCache, error) {
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
func (c *LRUCache) SetTransaction(ctx context.Context, tenantID string, txID string, data *domain.DataCache, ttl time.Duration) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.Set(ctx, tenantID, "tx:"+txID, bytes, ttl)
}

// IncrementCounter atomically increments a counter.
func (c *LRUCache) IncrementCounter(ctx context.Context, tenantID string, key string, window time.Duration) (int64, error) {
	if tenantID == "" {
		return 0, fmt.Errorf("tenantID is required")
	}

	fullKey := c.makeKey(tenantID, "counter:"+key)

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	entry, ok := c.counters[fullKey]

	if !ok || now.After(entry.expiresAt) {
		// Start new counter window
		c.counters[fullKey] = &counterEntry{
			count:     1,
			expiresAt: now.Add(window),
		}
		return 1, nil
	}

	entry.count++
	return entry.count, nil
}

// Ping checks cache health.
func (c *LRUCache) Ping(ctx context.Context) error {
	return nil
}

// Close cleans up the cache.
func (c *LRUCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.order = list.New()
	c.counters = make(map[string]*counterEntry)
	return nil
}

// Stats returns cache statistics.
func (c *LRUCache) Stats() (size int, capacity int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len(), c.maxSize
}

func (c *LRUCache) makeKey(tenantID, key string) string {
	return tenantID + ":" + key
}

func (c *LRUCache) removeElement(elem *list.Element) {
	c.order.Remove(elem)
	entry := elem.Value.(*cacheEntry)
	delete(c.items, entry.key)
}

func (c *LRUCache) removeOldest() {
	elem := c.order.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}
