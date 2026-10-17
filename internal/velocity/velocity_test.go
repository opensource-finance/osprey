package velocity

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/cache"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
)

func TestVelocityService(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "velocity-test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	// Create repository
	repo, err := repository.New(domain.RepositoryConfig{
		Driver:     "sqlite",
		SQLitePath: tmpPath,
	})
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create cache
	lruCache := cache.NewLRUCache(100)
	defer func() { _ = lruCache.Close() }()

	// Create velocity service
	svc := NewService(repo, lruCache)

	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("EmptyDatabase", func(t *testing.T) {
		count, err := svc.GetTransactionCount(ctx, tenantID, "user-001", 3600)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected count 0 for empty database, got %d", count)
		}
	})

	t.Run("WithTransactions", func(t *testing.T) {
		// Insert some transactions
		for i := range 5 {
			tx := &domain.Transaction{
				ID:              fmt.Sprintf("tx-%d", i),
				Type:            "transfer",
				DebtorID:        "user-001",
				DebtorAccountID: "acc-001",
				CreditorID:      "user-002",
				CreditorAcctID:  "acc-002",
				Amount:          100.0,
				Currency:        "USD",
				Timestamp:       time.Now().UTC(),
				CreatedAt:       time.Now().UTC(),
			}
			if err := repo.SaveTransaction(ctx, tenantID, tx); err != nil {
				t.Fatalf("failed to save transaction: %v", err)
			}
		}

		// Check debtor velocity
		count, err := svc.GetTransactionCount(ctx, tenantID, "user-001", 3600)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 5 {
			t.Errorf("expected count 5 for debtor, got %d", count)
		}

		// A creditor-only entity (user-002 only received funds, never sent) has
		// zero outgoing velocity: velocity_count is a debtor-side metric, so
		// incoming rows must not inflate it.
		count, err = svc.GetTransactionCount(ctx, tenantID, "user-002", 3600)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected count 0 for creditor-only entity (debtor-only), got %d", count)
		}

		// Check unknown user
		count, err = svc.GetTransactionCount(ctx, tenantID, "unknown-user", 3600)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected count 0 for unknown user, got %d", count)
		}
	})

	t.Run("TenantIsolation", func(t *testing.T) {
		// Different tenant should see 0
		count, err := svc.GetTransactionCount(ctx, "other-tenant", "user-001", 3600)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected count 0 for different tenant, got %d", count)
		}
	})

	t.Run("RequiresTenantID", func(t *testing.T) {
		_, err := svc.GetTransactionCount(ctx, "", "user-001", 3600)
		if err == nil {
			t.Error("expected error for empty tenantID")
		}
	})

	t.Run("RequiresEntityID", func(t *testing.T) {
		_, err := svc.GetTransactionCount(ctx, tenantID, "", 3600)
		if err == nil {
			t.Error("expected error for empty entityID")
		}
	})

	t.Run("VelocityGetter", func(t *testing.T) {
		getter := svc.GetVelocityGetter()
		if getter == nil {
			t.Fatal("GetVelocityGetter returned nil")
		}

		count, err := getter(ctx, tenantID, "user-001", 3600)
		if err != nil {
			t.Fatalf("VelocityGetter failed: %v", err)
		}
		if count != 5 {
			t.Errorf("expected count 5, got %d", count)
		}
	})
}

func TestNoDataSource(t *testing.T) {
	svc := &Service{} // No repo or db

	ctx := context.Background()
	_, err := svc.GetTransactionCount(ctx, "tenant", "entity", 3600)
	if err == nil {
		t.Error("expected error with no data source")
	}
}

// TestVelocityCountWindowsOnIngestTime checks the velocity window uses created_at, not timestamp.
func TestVelocityCountWindowsOnIngestTime(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "velocity-bypass-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()
	defer func() { _ = os.Remove(tmpPath + "-shm") }()
	defer func() { _ = os.Remove(tmpPath + "-wal") }()

	repo, err := repository.New(domain.RepositoryConfig{
		Driver:     "sqlite",
		SQLitePath: tmpPath,
	})
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	svc := NewService(repo, cache.NewLRUCache(100))
	ctx := context.Background()
	tenantID := "tenant-bypass"

	saveTx := func(id, debtor string, ts, createdAt time.Time) {
		t.Helper()
		tx := &domain.Transaction{
			ID:              id,
			Type:            "transfer",
			DebtorID:        debtor,
			DebtorAccountID: "acc-d",
			CreditorID:      "creditor-x",
			CreditorAcctID:  "acc-c",
			Amount:          100.0,
			Currency:        "USD",
			Timestamp:       ts.UTC(),
			CreatedAt:       createdAt.UTC(),
		}
		if err := repo.SaveTransaction(ctx, tenantID, tx); err != nil {
			t.Fatalf("SaveTransaction(%s) failed: %v", id, err)
		}
	}

	t.Run("BackdatedTimestampsStillCounted", func(t *testing.T) {
		// Backdated event timestamps still count.
		oldEventTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		now := time.Now().UTC()
		for i := range 6 {
			saveTx(fmt.Sprintf("bd-%d", i), "debtor-backdated", oldEventTime, now)
		}

		count, err := svc.GetTransactionCount(ctx, tenantID, "debtor-backdated", 3600)
		if err != nil {
			t.Fatalf("GetTransactionCount failed: %v", err)
		}
		if count != 6 {
			t.Fatalf("expected 6 backdated transactions counted by created_at (old code would read 0), got %d", count)
		}
	})

	t.Run("RecentTimestampButOldCreatedAtExcluded", func(t *testing.T) {
		// A recent timestamp with a stale created_at is excluded.
		now := time.Now().UTC()
		staleIngest := now.Add(-2 * time.Hour)
		for i := range 3 {
			saveTx(fmt.Sprintf("stale-%d", i), "debtor-stale-ca", now, staleIngest)
		}

		count, err := svc.GetTransactionCount(ctx, tenantID, "debtor-stale-ca", 3600)
		if err != nil {
			t.Fatalf("GetTransactionCount failed: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected 0 transactions (created_at outside window), got %d (window keyed on wrong column)", count)
		}
	})

	t.Run("WindowBoundaryOnCreatedAt", func(t *testing.T) {
		// Only the transaction ingested inside the window counts.
		oldEventTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		now := time.Now().UTC()
		saveTx("boundary-in", "debtor-boundary", oldEventTime, now.Add(-30*time.Second))
		saveTx("boundary-out", "debtor-boundary", oldEventTime, now.Add(-2*time.Hour))

		count, err := svc.GetTransactionCount(ctx, tenantID, "debtor-boundary", 3600)
		if err != nil {
			t.Fatalf("GetTransactionCount failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 transaction within the created_at window, got %d", count)
		}
	})
}

// TestVelocityHugeWindowDoesNotOverflow covers issue #64. A window above about
// 9.2e9 seconds overflowed time.Duration, put the window start in the future,
// and returned a count of 0, so velocity rules never fired.
func TestVelocityHugeWindowDoesNotOverflow(t *testing.T) {
	tmpPath := t.TempDir() + "/velocity-overflow.db"
	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: tmpPath})
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	svc := NewService(repo, cache.NewLRUCache(100))
	ctx := context.Background()
	now := time.Now().UTC()
	for i := range 3 {
		tx := &domain.Transaction{
			ID:              fmt.Sprintf("huge-%d", i),
			Type:            "transfer",
			DebtorID:        "debtor-huge",
			DebtorAccountID: "acc-d",
			CreditorID:      "creditor-x",
			CreditorAcctID:  "acc-c",
			Amount:          100.0,
			Currency:        "USD",
			Timestamp:       now,
			CreatedAt:       now,
		}
		if err := repo.SaveTransaction(ctx, "tenant-huge", tx); err != nil {
			t.Fatalf("SaveTransaction failed: %v", err)
		}
	}

	for _, window := range []int{9223372037, 10000000000, math.MaxInt} {
		count, err := svc.GetTransactionCount(ctx, "tenant-huge", "debtor-huge", window)
		if err != nil {
			t.Fatalf("GetTransactionCount(%d) failed: %v", window, err)
		}
		if count != 3 {
			t.Errorf("GetTransactionCount(window=%d) = %d, want 3", window, count)
		}
		agg, err := svc.GetAggregates(ctx, "tenant-huge", "debtor-huge", window)
		if err != nil {
			t.Fatalf("GetAggregates(%d) failed: %v", window, err)
		}
		if agg.Count != 3 || agg.AmountSum != 300 {
			t.Errorf("GetAggregates(window=%d) = %+v, want Count 3 and AmountSum 300", window, agg)
		}
	}
}

func TestWindowStartClampsToMax(t *testing.T) {
	got := time.Since(windowStart(math.MaxInt))
	want := time.Duration(MaxWindowSecs) * time.Second
	if got < want || got > want+time.Minute {
		t.Errorf("windowStart(MaxInt) is %v ago, want about %v", got, want)
	}
}
