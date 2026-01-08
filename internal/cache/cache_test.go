package cache

import (
	"context"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestLRUCache(t *testing.T) {
	cache := NewLRUCache(100)
	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("SetAndGet", func(t *testing.T) {
		err := cache.Set(ctx, tenantID, "key1", []byte("value1"), time.Minute)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		val, err := cache.Get(ctx, tenantID, "key1")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if string(val) != "value1" {
			t.Errorf("expected 'value1', got '%s'", string(val))
		}
	})

	t.Run("GetMiss", func(t *testing.T) {
		val, err := cache.Get(ctx, tenantID, "nonexistent")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if val != nil {
			t.Errorf("expected nil for cache miss, got: %v", val)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		_ = cache.Set(ctx, tenantID, "key2", []byte("value2"), time.Minute)

		err := cache.Delete(ctx, tenantID, "key2")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		val, _ := cache.Get(ctx, tenantID, "key2")
		if val != nil {
			t.Error("expected nil after delete")
		}
	})

	t.Run("TTLExpiration", func(t *testing.T) {
		_ = cache.Set(ctx, tenantID, "expiring", []byte("temp"), 10*time.Millisecond)

		// Should be available immediately
		val, _ := cache.Get(ctx, tenantID, "expiring")
		if val == nil {
			t.Error("expected value before expiration")
		}

		// Wait for expiration
		time.Sleep(20 * time.Millisecond)

		val, _ = cache.Get(ctx, tenantID, "expiring")
		if val != nil {
			t.Error("expected nil after expiration")
		}
	})

	t.Run("LRUEviction", func(t *testing.T) {
		smallCache := NewLRUCache(3)

		_ = smallCache.Set(ctx, tenantID, "a", []byte("1"), time.Minute)
		_ = smallCache.Set(ctx, tenantID, "b", []byte("2"), time.Minute)
		_ = smallCache.Set(ctx, tenantID, "c", []byte("3"), time.Minute)

		// Access 'a' to make it recently used
		_, _ = smallCache.Get(ctx, tenantID, "a")

		// Add 'd' - should evict 'b' (oldest accessed)
		_ = smallCache.Set(ctx, tenantID, "d", []byte("4"), time.Minute)

		// 'b' should be evicted
		val, _ := smallCache.Get(ctx, tenantID, "b")
		if val != nil {
			t.Error("expected 'b' to be evicted")
		}

		// 'a' should still be there
		val, _ = smallCache.Get(ctx, tenantID, "a")
		if val == nil {
			t.Error("expected 'a' to still exist")
		}
	})

	t.Run("TenantIsolation", func(t *testing.T) {
		tenant1 := "tenant-001"
		tenant2 := "tenant-002"

		_ = cache.Set(ctx, tenant1, "shared-key", []byte("tenant1-value"), time.Minute)
		_ = cache.Set(ctx, tenant2, "shared-key", []byte("tenant2-value"), time.Minute)

		val1, _ := cache.Get(ctx, tenant1, "shared-key")
		val2, _ := cache.Get(ctx, tenant2, "shared-key")

		if string(val1) != "tenant1-value" {
			t.Errorf("expected 'tenant1-value', got '%s'", string(val1))
		}
		if string(val2) != "tenant2-value" {
			t.Errorf("expected 'tenant2-value', got '%s'", string(val2))
		}
	})

	t.Run("RequiresTenantID", func(t *testing.T) {
		err := cache.Set(ctx, "", "key", []byte("value"), time.Minute)
		if err == nil {
			t.Error("expected error for empty tenantID")
		}

		_, err = cache.Get(ctx, "", "key")
		if err == nil {
			t.Error("expected error for empty tenantID")
		}
	})

	t.Run("IncrementCounter", func(t *testing.T) {
		window := 100 * time.Millisecond

		count1, err := cache.IncrementCounter(ctx, tenantID, "velocity", window)
		if err != nil {
			t.Fatalf("IncrementCounter failed: %v", err)
		}
		if count1 != 1 {
			t.Errorf("expected count 1, got %d", count1)
		}

		count2, _ := cache.IncrementCounter(ctx, tenantID, "velocity", window)
		if count2 != 2 {
			t.Errorf("expected count 2, got %d", count2)
		}

		// Wait for window to expire
		time.Sleep(150 * time.Millisecond)

		count3, _ := cache.IncrementCounter(ctx, tenantID, "velocity", window)
		if count3 != 1 {
			t.Errorf("expected count 1 after window reset, got %d", count3)
		}
	})

	t.Run("TransactionCache", func(t *testing.T) {
		data := &domain.DataCache{
			DebtorID:   "debtor-001",
			CreditorID: "creditor-001",
			Amount:     1000.50,
			Currency:   "USD",
		}

		err := cache.SetTransaction(ctx, tenantID, "tx-001", data, time.Minute)
		if err != nil {
			t.Fatalf("SetTransaction failed: %v", err)
		}

		retrieved, err := cache.GetTransaction(ctx, tenantID, "tx-001")
		if err != nil {
			t.Fatalf("GetTransaction failed: %v", err)
		}

		if retrieved.DebtorID != data.DebtorID {
			t.Errorf("expected DebtorID %s, got %s", data.DebtorID, retrieved.DebtorID)
		}
		if retrieved.Amount != data.Amount {
			t.Errorf("expected Amount %.2f, got %.2f", data.Amount, retrieved.Amount)
		}
	})

	t.Run("Stats", func(t *testing.T) {
		statsCache := NewLRUCache(50)
		_ = statsCache.Set(ctx, tenantID, "k1", []byte("v1"), time.Minute)
		_ = statsCache.Set(ctx, tenantID, "k2", []byte("v2"), time.Minute)

		size, capacity := statsCache.Stats()
		if size != 2 {
			t.Errorf("expected size 2, got %d", size)
		}
		if capacity != 50 {
			t.Errorf("expected capacity 50, got %d", capacity)
		}
	})

	t.Run("Ping", func(t *testing.T) {
		if err := cache.Ping(ctx); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	t.Run("Close", func(t *testing.T) {
		testCache := NewLRUCache(10)
		_ = testCache.Set(ctx, tenantID, "k", []byte("v"), time.Minute)

		err := testCache.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}

		// Cache should be empty after close
		val, _ := testCache.Get(ctx, tenantID, "k")
		if val != nil {
			t.Error("expected cache to be cleared after close")
		}
	})
}

func TestNewCache(t *testing.T) {
	t.Run("MemoryType", func(t *testing.T) {
		cfg := domain.CacheConfig{
			Type:         "memory",
			LocalMaxSize: 100,
		}

		cache, err := New(cfg)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		defer cache.Close()

		_, ok := cache.(*LRUCache)
		if !ok {
			t.Error("expected LRUCache for memory type")
		}
	})

	t.Run("UnsupportedType", func(t *testing.T) {
		cfg := domain.CacheConfig{
			Type: "memcached",
		}

		_, err := New(cfg)
		if err == nil {
			t.Error("expected error for unsupported type")
		}
	})
}
