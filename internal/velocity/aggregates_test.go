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
		agg, err := svc.GetAggregates(ctx, tenantID, "d1", "USD", 3600)
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
		if _, err := svc.GetAggregates(ctx, "", "d1", "USD", 3600); err == nil {
			t.Error("expected error for empty tenantID")
		}
		if _, err := svc.GetAggregates(ctx, tenantID, "", "USD", 3600); err == nil {
			t.Error("expected error for empty entityID")
		}
	})

	t.Run("NoDataSource", func(t *testing.T) {
		if _, err := (&Service{}).GetAggregates(ctx, "t", "e", "USD", 3600); err == nil {
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

	agg, err := svc.GetAggregates(ctx, tenantID, "d1-bd", "USD", 3600)
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

	agg, err := svc.GetAggregates(ctx, "t", "mule", "GBP", 3600)
	if err != nil {
		t.Fatalf("GetAggregates: %v", err)
	}
	want := Aggregates{Count: 3, AmountSum: 3800, DistinctCreditors: 1, InboundCount: 2, InboundAmountSum: 3000}
	if agg != want {
		t.Errorf("got %+v, want %+v", agg, want)
	}
}

// TestAggregateSumsPerCurrency is the regression test for currency-incoherent
// velocity sums. Before the fix, aggregate added raw amounts across every
// currency in the window, so one 100 EUR outbound tx and one 100 USD inbound tx
// for the same entity produced AmountSum=200 (neither 200 USD nor 200 EUR).
// The sums must now be scoped to the evaluated transaction's currency, while the
// dimensionless counters stay currency-agnostic.
func TestAggregateSumsPerCurrency(t *testing.T) {
	entity := "E"
	now := time.Now().UTC()
	txs := []*domain.Transaction{
		{ID: "out-eur", Type: "TRANSFER", DebtorID: entity, CreditorID: "c1", Amount: 100, Currency: "EUR", Timestamp: now, CreatedAt: now},
		{ID: "in-usd", Type: "TRANSFER", DebtorID: "other", CreditorID: entity, Amount: 100, Currency: "USD", Timestamp: now, CreatedAt: now},
	}

	// Counts cover every transaction regardless of currency.
	aggUSD := aggregate(entity, "USD", txs)
	if aggUSD.Count != 2 {
		t.Errorf("Count: want 2 (currency-agnostic), got %d", aggUSD.Count)
	}
	if aggUSD.InboundCount != 1 {
		t.Errorf("InboundCount: want 1 (currency-agnostic), got %d", aggUSD.InboundCount)
	}
	// USD sums only the USD inbound row; the EUR outbound row is excluded.
	if aggUSD.AmountSum != 100 {
		t.Errorf("USD AmountSum: want 100.0, got %f", aggUSD.AmountSum)
	}
	if aggUSD.InboundAmountSum != 100 {
		t.Errorf("USD InboundAmountSum: want 100.0, got %f", aggUSD.InboundAmountSum)
	}

	// EUR sums only the EUR outbound row; the USD inbound row is excluded.
	aggEUR := aggregate(entity, "EUR", txs)
	if aggEUR.AmountSum != 100 {
		t.Errorf("EUR AmountSum: want 100.0, got %f", aggEUR.AmountSum)
	}
	if aggEUR.InboundAmountSum != 0 {
		t.Errorf("EUR InboundAmountSum: want 0.0, got %f", aggEUR.InboundAmountSum)
	}
	if aggEUR.DistinctCreditors != 1 {
		t.Errorf("EUR DistinctCreditors: want 1, got %d", aggEUR.DistinctCreditors)
	}

	// A third currency present in the window contributes nothing to the sums.
	aggJPY := aggregate(entity, "JPY", txs)
	if aggJPY.AmountSum != 0 || aggJPY.InboundAmountSum != 0 {
		t.Errorf("JPY sums should be 0, got %+v", aggJPY)
	}
	if aggJPY.Count != 2 {
		t.Errorf("JPY Count: want 2 (currency-agnostic), got %d", aggJPY.Count)
	}

	// Empty currency falls back to summing every row (legacy behavior), so a
	// caller that does not know the currency is no worse off than before.
	aggAll := aggregate(entity, "", txs)
	if aggAll.AmountSum != 200 {
		t.Errorf("empty-currency AmountSum: want 200.0 (legacy fallback), got %f", aggAll.AmountSum)
	}
	if aggAll.InboundAmountSum != 100 {
		t.Errorf("empty-currency InboundAmountSum: want 100.0 (legacy fallback), got %f", aggAll.InboundAmountSum)
	}
}

// TestGetAggregatesPerCurrencyEndToEnd checks the per-currency sums through the
// service and repository, the path the rule engine uses.
func TestGetAggregatesPerCurrencyEndToEnd(t *testing.T) {
	svc, repo := newAggRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	save := func(id, from, to, currency string, amount float64) {
		t.Helper()
		tx := &domain.Transaction{ID: id, Type: "TRANSFER", DebtorID: from, CreditorID: to, Amount: amount, Currency: currency, Timestamp: now, CreatedAt: now}
		if err := repo.SaveTransaction(ctx, "t", tx); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	// Entity mule sends 100 EUR and 300 USD, and receives 200 USD.
	save("out-eur", "mule", "x1", "EUR", 100)
	save("out-usd", "mule", "x2", "USD", 300)
	save("in-usd", "victim", "mule", "USD", 200)

	agg, err := svc.GetAggregates(ctx, "t", "mule", "USD", 3600)
	if err != nil {
		t.Fatalf("GetAggregates: %v", err)
	}
	// Count is currency-agnostic: all three txs.
	if agg.Count != 3 {
		t.Errorf("Count: want 3, got %d", agg.Count)
	}
	// USD sum: 300 sent + 200 received = 500. The 100 EUR row is excluded.
	if agg.AmountSum != 500 {
		t.Errorf("USD AmountSum: want 500.0, got %f", agg.AmountSum)
	}
	if agg.InboundAmountSum != 200 {
		t.Errorf("USD InboundAmountSum: want 200.0, got %f", agg.InboundAmountSum)
	}
	if agg.InboundCount != 1 {
		t.Errorf("USD InboundCount: want 1, got %d", agg.InboundCount)
	}
	if agg.DistinctCreditors != 2 {
		t.Errorf("DistinctCreditors: want 2, got %d", agg.DistinctCreditors)
	}

	// EUR sum: only the 100 EUR outbound row.
	aggEUR, err := svc.GetAggregates(ctx, "t", "mule", "EUR", 3600)
	if err != nil {
		t.Fatalf("GetAggregates EUR: %v", err)
	}
	if aggEUR.AmountSum != 100 {
		t.Errorf("EUR AmountSum: want 100.0, got %f", aggEUR.AmountSum)
	}
	if aggEUR.InboundAmountSum != 0 {
		t.Errorf("EUR InboundAmountSum: want 0.0, got %f", aggEUR.InboundAmountSum)
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

	agg, err := svc.GetAggregatesAt(ctx, "t", "d", "USD", 3600, base)
	if err != nil {
		t.Fatalf("GetAggregatesAt: %v", err)
	}
	// In the hour that ends at base: -30m and 0. Not -3h (too old) or +1h (in the future).
	if agg.Count != 2 || agg.DistinctCreditors != 2 {
		t.Errorf("event window: want count 2 and 2 payees, got %+v", agg)
	}

	// The received clock sees all 4: Osprey received them all just now.
	recv, err := svc.GetAggregates(ctx, "t", "d", "USD", 3600)
	if err != nil {
		t.Fatalf("GetAggregates: %v", err)
	}
	if recv.Count != 4 {
		t.Errorf("received window: want count 4, got %d", recv.Count)
	}
}

func TestAggregateCurrencyIgnoresCase(t *testing.T) {
	now := time.Now().UTC()
	txs := []*domain.Transaction{
		{ID: "in", DebtorID: "other", CreditorID: "E", Amount: 100, Currency: "USD", Timestamp: now, CreatedAt: now},
	}
	// The async path can pass the currency as the producer sent it.
	if got := aggregate("E", "usd", txs).InboundAmountSum; got != 100 {
		t.Errorf("lower-case currency: InboundAmountSum = %f, want 100", got)
	}
}
