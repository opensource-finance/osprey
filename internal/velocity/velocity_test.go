package velocity

import (
	"context"
	"fmt"
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
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Create repository
	repo, err := repository.New(domain.RepositoryConfig{
		Driver:     "sqlite",
		SQLitePath: tmpPath,
	})
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer repo.Close()

	// Create cache
	lruCache := cache.NewLRUCache(100)
	defer lruCache.Close()

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
		for i := 0; i < 5; i++ {
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

		// Check creditor velocity
		count, err = svc.GetTransactionCount(ctx, tenantID, "user-002", 3600)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 5 {
			t.Errorf("expected count 5 for creditor, got %d", count)
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
