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

// TestGetAggregatesBackdatedTimestamps checks backdated timestamps cannot zero out the aggregates.
func TestGetAggregatesBackdatedTimestamps(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "velagg-bypass-*.db")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()
	defer func() { _ = os.Remove(tmpPath + "-shm") }()
	defer func() { _ = os.Remove(tmpPath + "-wal") }()

	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: tmpPath})
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	defer func() { _ = repo.Close() }()

	svc := NewService(repo, cache.NewLRUCache(100))
	ctx := context.Background()
	tenantID := "t1-bypass"

	// 5 backdated txs to 2 creditors: count 5, sum 500, 2 distinct creditors.
	oldEventTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Now().UTC()
	creditors := []string{"c1-bd", "c1-bd", "c1-bd", "c2-bd", "c2-bd"}
	for i, c := range creditors {
		tx := &domain.Transaction{
			ID: fmt.Sprintf("abd-%d", i), Type: "transfer",
			DebtorID: "d1-bd", CreditorID: c, Amount: 100.0, Currency: "USD",
			Timestamp: oldEventTime, CreatedAt: now,
		}
		if err := repo.SaveTransaction(ctx, tenantID, tx); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	agg, err := svc.GetAggregates(ctx, tenantID, "d1-bd", 3600)
	if err != nil {
		t.Fatalf("GetAggregates: %v", err)
	}
	if agg.Count != 5 {
		t.Errorf("count: want 5 (created_at in window), got %d (old code would read 0)", agg.Count)
	}
	if agg.AmountSum != 500.0 {
		t.Errorf("amountSum: want 500.0, got %.2f", agg.AmountSum)
	}
	if agg.DistinctCreditors != 2 {
		t.Errorf("distinctCreditors: want 2, got %d", agg.DistinctCreditors)
	}
}

// newAggRepo returns a fresh SQLite-backed velocity service and its repository.
func newAggRepo(t *testing.T) (*Service, domain.Repository) {
	t.Helper()
	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: t.TempDir() + "/v.db"})
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return NewService(repo, cache.NewLRUCache(100)), repo
}

// TestDistinctCreditorsCountsOutgoingOnly is the regression test for an account
// that received money: the incoming payment must not add the account itself as a payee.
func TestDistinctCreditorsCountsOutgoingOnly(t *testing.T) {
	svc, repo := newAggRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	save := func(id, from, to string, amount float64) {
		t.Helper()
		tx := &domain.Transaction{ID: id, Type: "TRANSFER", DebtorID: from, CreditorID: to, Amount: amount, Currency: "GBP", Timestamp: now, CreatedAt: now}
		if err := repo.SaveTransaction(ctx, "t", tx); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	save("in-1", "victim", "mule", 2500)
	save("in-2", "victim-2", "mule", 500)
	save("out-1", "mule", "x1", 800)

	agg, err := svc.GetAggregates(ctx, "t", "mule", 3600)
	if err != nil {
		t.Fatalf("GetAggregates: %v", err)
	}
	want := Aggregates{Count: 3, AmountSum: 3800, DistinctCreditors: 1, InboundCount: 2, InboundAmountSum: 3000}
	if agg != want {
		t.Errorf("got %+v, want %+v", agg, want)
	}
}

// TestGetAggregatesAtUsesEventTime checks the event clock: the window ends at the
// given event time and is keyed on each transaction's own timestamp.
func TestGetAggregatesAtUsesEventTime(t *testing.T) {
	svc, repo := newAggRepo(t)
	ctx := context.Background()
	base := time.Date(2017, 3, 1, 0, 0, 0, 0, time.UTC)
	now := time.Now().UTC()
	for i, offset := range []time.Duration{-3 * time.Hour, -30 * time.Minute, 0, time.Hour} {
		tx := &domain.Transaction{ID: fmt.Sprintf("e-%d", i), Type: "TRANSFER", DebtorID: "d", CreditorID: fmt.Sprintf("c%d", i),
			Amount: 10, Currency: "USD", Timestamp: base.Add(offset), CreatedAt: now}
		if err := repo.SaveTransaction(ctx, "t", tx); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	agg, err := svc.GetAggregatesAt(ctx, "t", "d", 3600, base)
	if err != nil {
		t.Fatalf("GetAggregatesAt: %v", err)
	}
	// In the hour that ends at base: -30m and 0. Not -3h (too old) or +1h (in the future).
	if agg.Count != 2 || agg.DistinctCreditors != 2 {
		t.Errorf("event window: want count 2 and 2 payees, got %+v", agg)
	}

	// The received clock sees all 4: Osprey received them all just now.
	recv, err := svc.GetAggregates(ctx, "t", "d", 3600)
	if err != nil {
		t.Fatalf("GetAggregates: %v", err)
	}
	if recv.Count != 4 {
		t.Errorf("received window: want count 4, got %d", recv.Count)
	}
}
