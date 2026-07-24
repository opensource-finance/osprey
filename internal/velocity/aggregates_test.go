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

func TestGetAggregates(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "velagg-test-*.db")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: tmpPath})
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	defer func() { _ = repo.Close() }()

	svc := NewService(repo, cache.NewLRUCache(100))
	ctx := context.Background()
	tenantID := "t1"

	// d1 -> c1 x3, d1 -> c2 x2, all 100.0 (count 5, sum 500, distinct creditors 2)
	creditors := []string{"c1", "c1", "c1", "c2", "c2"}
	for i, c := range creditors {
		tx := &domain.Transaction{
			ID: fmt.Sprintf("a-%d", i), Type: "transfer",
			DebtorID: "d1", CreditorID: c, Amount: 100.0, Currency: "USD",
			Timestamp: time.Now().UTC(), CreatedAt: time.Now().UTC(),
		}
		if err := repo.SaveTransaction(ctx, tenantID, tx); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	t.Run("Aggregates", func(t *testing.T) {
		agg, err := svc.GetAggregates(ctx, tenantID, "d1", 3600)
		if err != nil {
			t.Fatalf("GetAggregates: %v", err)
		}
		if agg.Count != 5 {
			t.Errorf("count: want 5, got %d", agg.Count)
		}
		if agg.AmountSum != 500.0 {
			t.Errorf("amountSum: want 500.0, got %.2f", agg.AmountSum)
		}
		if agg.DistinctCreditors != 2 {
			t.Errorf("distinctCreditors: want 2, got %d", agg.DistinctCreditors)
		}
	})

	t.Run("RequiresIDs", func(t *testing.T) {
		if _, err := svc.GetAggregates(ctx, "", "d1", 3600); err == nil {
			t.Error("expected error for empty tenantID")
		}
		if _, err := svc.GetAggregates(ctx, tenantID, "", 3600); err == nil {
			t.Error("expected error for empty entityID")
		}
	})

	t.Run("NoDataSource", func(t *testing.T) {
		if _, err := (&Service{}).GetAggregates(ctx, "t", "e", 3600); err == nil {
			t.Error("expected error with no data source")
		}
	})
}
