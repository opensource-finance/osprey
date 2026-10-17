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

// TestVelocityIgnoresIncomingTransactions is the regression test for the
// bidirectional-velocity bug: GetTransactionsByEntity matches the entity in
// either role (debtor_id = ? OR creditor_id = ?), so incoming payments must
// not inflate the debtor's velocity_count / velocity_amount_sum, and the
// entity must not count as its own creditor in velocity_distinct_creditors.
//
// Seed: d1 sends to two creditors and also receives from several senders.
// debtor-only expectation for d1: count 2, amountSum 150.0, 2 distinct
// creditors (c1, c2). The incoming rows (where creditor_id == d1) must be
// ignored, and d1 itself must not appear in the distinct-creditor set.
func TestVelocityIgnoresIncomingTransactions(t *testing.T) {
	svc, ctx, tenantID := newDirectionalFixture(t)

	for _, tx := range []struct {
		id, debtor, creditor string
		amount               float64
	}{
		{"out-1", "d1", "c1", 100.0}, // outgoing from d1
		{"out-2", "d1", "c2", 50.0},  // outgoing from d1
		{"in-1", "c9", "d1", 1000.0}, // incoming to d1
		{"in-2", "c8", "d1", 2000.0}, // incoming to d1
		{"in-3", "c9", "d1", 777.0},  // incoming to d1
	} {
		saveDirectionalTx(t, svc.repo, ctx, tenantID, tx.id, tx.debtor, tx.creditor, tx.amount)
	}

	// Debtor-only expectations for d1: incoming rows must not leak in and d1
	// must not be counted as its own creditor.
	t.Run("AggregatesAreDebtorOnly", func(t *testing.T) {
		assertAggregates(t, svc, ctx, tenantID, "d1", Aggregates{Count: 2, AmountSum: 150.0, DistinctCreditors: 2})
	})

	t.Run("CountIsDebtorOnly", func(t *testing.T) {
		assertCount(t, svc, ctx, tenantID, "d1", 2)
	})

	t.Run("CreditorOnlyEntityHasZeroVelocity", func(t *testing.T) {
		// c1 and c2 only ever received funds from d1; they never sent.
		for _, creditor := range []string{"c1", "c2"} {
			assertCount(t, svc, ctx, tenantID, creditor, 0)
			assertAggregates(t, svc, ctx, tenantID, creditor, Aggregates{})
		}
	})

	t.Run("IncomingRowsSendersKeepOwnVelocity", func(t *testing.T) {
		// The senders of the incoming rows (c9, c8) keep their own outgoing
		// velocity, proving the filter is directional, not a blanket drop.
		assertAggregates(t, svc, ctx, tenantID, "c9", Aggregates{Count: 2, AmountSum: 1777.0, DistinctCreditors: 1})
		assertAggregates(t, svc, ctx, tenantID, "c8", Aggregates{Count: 1, AmountSum: 2000.0, DistinctCreditors: 1})
	})
}

// assertAggregates fails the test unless GetAggregates returns want for entity.
func assertAggregates(t *testing.T, svc *Service, ctx context.Context, tenantID, entityID string, want Aggregates) {
	t.Helper()
	got, err := svc.GetAggregates(ctx, tenantID, entityID, 3600)
	if err != nil {
		t.Fatalf("GetAggregates(%s): %v", entityID, err)
	}
	if got != want {
		t.Errorf("%s aggregates: want %+v, got %+v", entityID, want, got)
	}
}

// assertCount fails the test unless GetTransactionCount returns want for entity.
func assertCount(t *testing.T, svc *Service, ctx context.Context, tenantID, entityID string, want int64) {
	t.Helper()
	got, err := svc.GetTransactionCount(ctx, tenantID, entityID, 3600)
	if err != nil {
		t.Fatalf("GetTransactionCount(%s): %v", entityID, err)
	}
	if got != want {
		t.Errorf("%s count: want %d, got %d", entityID, want, got)
	}
}

// newDirectionalFixture builds a fresh SQLite-backed velocity service for the
// mixed-direction regression tests.
func newDirectionalFixture(t *testing.T) (*Service, context.Context, string) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "velagg-dir-*.db")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	t.Cleanup(func() {
		_ = os.Remove(tmpPath)
		_ = os.Remove(tmpPath + "-shm")
		_ = os.Remove(tmpPath + "-wal")
	})

	repo, err := repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: tmpPath})
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	return NewService(repo, cache.NewLRUCache(100)), context.Background(), "t1-dir"
}

// saveDirectionalTx stores a transaction in the directional fixture.
func saveDirectionalTx(t *testing.T, repo domain.Repository, ctx context.Context, tenantID, id, debtor, creditor string, amount float64) {
	t.Helper()
	now := time.Now().UTC()
	tx := &domain.Transaction{
		ID: id, Type: "transfer", DebtorID: debtor, CreditorID: creditor,
		Amount: amount, Currency: "USD", Timestamp: now, CreatedAt: now,
	}
	if err := repo.SaveTransaction(ctx, tenantID, tx); err != nil {
		t.Fatalf("save %s: %v", id, err)
	}
}

// TestVelocityExploitIncomingDoesNotInflate reproduces the advisory scenario: an
// attacker floods a victim's entity ID with fake incoming payments so the
// victim's legitimate outgoing transaction gets flagged. Under the fix the
// attacker traffic must not move the victim's velocity_* metrics.
func TestVelocityExploitIncomingDoesNotInflate(t *testing.T) {
	svc, ctx, tenantID := newDirectionalFixture(t)
	victim := "victim"

	saveDirectionalTx(t, svc.repo, ctx, tenantID, "legit", victim, "shop", 100.0)
	// Attacker floods the victim with many small incoming payments from distinct
	// fake debtor IDs, trying to trip "velocity_distinct_creditors > 5" and
	// "velocity_amount_sum > 10000" on the victim's next outgoing payment.
	for i := range 10 {
		saveDirectionalTx(t, svc.repo, ctx, tenantID, fmt.Sprintf("attack-%d", i), fmt.Sprintf("attacker-%d", i), victim, 2000.0)
	}

	agg, err := svc.GetAggregates(ctx, tenantID, victim, 3600)
	if err != nil {
		t.Fatalf("GetAggregates(victim): %v", err)
	}
	want := Aggregates{Count: 1, AmountSum: 100.0, DistinctCreditors: 1}
	if agg != want {
		t.Errorf("exploit: victim want %+v (legit outgoing only), got %+v (incoming attack leaked in)", want, agg)
	}
}
