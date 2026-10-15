package repository

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/opensource-finance/osprey/internal/domain"
)

// postgresTestConfig builds a RepositoryConfig for the pro-tier (PostgreSQL)
// driver from OSPREY_POSTGRES_TEST_* env vars, defaulting to the
// docker-compose.yml postgres:16-alpine service values. Returns ok=false when
// the suite has not been opted into (no host configured).
func postgresTestConfig(t *testing.T) (domain.RepositoryConfig, bool) {
	t.Helper()
	host := os.Getenv("OSPREY_POSTGRES_TEST_HOST")
	if host == "" {
		return domain.RepositoryConfig{}, false
	}
	if testing.Short() {
		return domain.RepositoryConfig{}, false
	}

	port := 5432
	if v := os.Getenv("OSPREY_POSTGRES_TEST_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			port = p
		}
	}
	user := os.Getenv("OSPREY_POSTGRES_TEST_USER")
	if user == "" {
		user = "osprey"
	}
	password := os.Getenv("OSPREY_POSTGRES_TEST_PASSWORD")
	if password == "" {
		password = "osprey_secret"
	}
	dbname := os.Getenv("OSPREY_POSTGRES_TEST_DB")
	if dbname == "" {
		dbname = "osprey"
	}
	sslmode := os.Getenv("OSPREY_POSTGRES_TEST_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}

	return domain.RepositoryConfig{
		Driver:           "postgres",
		PostgresHost:     host,
		PostgresPort:     port,
		PostgresUser:     user,
		PostgresPassword: password,
		PostgresDB:       dbname,
		PostgresSSLMode:  sslmode,
	}, true
}

// TestPostgresREALColumnsAreDoublePrecision is a live-PostgreSQL regression
// test for the float4 truncation bug. It is skipped unless
// OSPREY_POSTGRES_TEST_HOST is set (and -short is not passed), so it never
// breaks the default unit-test run. Bring up the docker-compose postgres
// service and run:
//
//	OSPREY_POSTGRES_TEST_HOST=localhost go test -run TestPostgres ./internal/repository/...
//
// Preconditions: a reachable PostgreSQL 16 server matching the configured
// OSPREY_POSTGRES_TEST_* connection (the docker-compose.yml `postgres`
// service works out of the box). The database must be writable; the test
// uses a unique tenant per run so it never collides with prior runs.
func TestPostgresREALColumnsAreDoublePrecision(t *testing.T) {
	cfg, ok := postgresTestConfig(t)
	if !ok {
		t.Skip("set OSPREY_POSTGRES_TEST_HOST (and stand up a PostgreSQL server) to run the live Postgres regression test")
	}

	repo, err := New(cfg)
	if err != nil {
		t.Fatalf("New(postgres) failed (is the server reachable?): %v", err)
	}
	defer func() { _ = repo.Close() }()

	sqlRepo := repo.(*SQLRepository)

	// Every column that the shared DDL declares as REAL must materialise as
	// PostgreSQL `double precision` (float8), not `real` (float4). This is the
	// root-cause assertion: the transformForPostgres REAL -> DOUBLE PRECISION
	// rewrite is what makes these columns float8.
	columns := []struct {
		table  string
		column string
	}{
		{"transactions", "amount"},
		{"rule_configs", "weight"},
		{"evaluations", "score"},
		{"typologies", "alert_threshold"},
	}
	for _, c := range columns {
		var dataType string
		err := sqlRepo.db.QueryRow(
			`SELECT data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`,
			c.table, c.column,
		).Scan(&dataType)
		if err != nil {
			t.Errorf("could not introspect %s.%s: %v", c.table, c.column, err)
			continue
		}
		if dataType != "double precision" {
			t.Errorf("%s.%s is %q on PostgreSQL; expected \"double precision\" (float8) to avoid float4 truncation", c.table, c.column, dataType)
		}
	}
}

// TestPostgresAmountRoundTripPreservesPrecision verifies the end-to-end
// manifestation of the bug: a monetary amount with more than ~7 significant
// digits must survive a write+read round-trip exactly. With float4 (the bug),
// 1234567.89 is stored/returned as 1234567.9 and 999999.99 as 1000000.
func TestPostgresAmountRoundTripPreservesPrecision(t *testing.T) {
	cfg, ok := postgresTestConfig(t)
	if !ok {
		t.Skip("set OSPREY_POSTGRES_TEST_HOST (and stand up a PostgreSQL server) to run the live Postgres regression test")
	}

	repo, err := New(cfg)
	if err != nil {
		t.Fatalf("New(postgres) failed (is the server reachable?): %v", err)
	}
	defer func() { _ = repo.Close() }()

	sqlRepo := repo.(*SQLRepository)

	ctx := context.Background()
	tenant := fmt.Sprintf("pg-precision-%d", time.Now().UnixNano())

	// 1234567.89 has 9 significant digits; float4 rounds it to 1234567.9.
	// 999999.99 has 8 significant digits; float4 rounds it to 1000000.
	amounts := []float64{1234567.89, 999999.99, 12345678.90}

	for i, amount := range amounts {
		txID := fmt.Sprintf("pg-amount-%d", i)
		tx := &domain.Transaction{ //nolint:gosec // G101: synthetic test identifiers, not credentials
			ID:              txID,
			Type:            "transfer",
			DebtorID:        "debtor-pg",
			DebtorAccountID: "acc-pg-d",
			CreditorID:      "creditor-pg",
			CreditorAcctID:  "acc-pg-c",
			Amount:          amount,
			Currency:        "USD",
			Timestamp:       time.Now().UTC(),
			CreatedAt:       time.Now().UTC(),
		}
		if err := repo.SaveTransaction(ctx, tenant, tx); err != nil {
			t.Fatalf("SaveTransaction(%s) failed: %v", txID, err)
		}

		got, err := repo.GetTransaction(ctx, tenant, txID)
		if err != nil {
			t.Fatalf("GetTransaction(%s) failed: %v", txID, err)
		}
		if got.Amount != amount {
			t.Errorf("amount round-trip corrupted via GetTransaction: wrote %v, read %v (float4 truncation regression)", amount, got.Amount)
		}
	}

	// The velocity path (GetTransactionsByEntity) loads the same repo-stored
	// amount into tx.Amount, which velocity aggregates as AmountSum and
	// exposes to CEL rules. Results are ordered by created_at DESC, so compare
	// the returned amounts as a multiset rather than positionally.
	entityTx, err := repo.GetTransactionsByEntity(ctx, tenant, "debtor-pg", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("GetTransactionsByEntity failed: %v", err)
	}
	if len(entityTx) != len(amounts) {
		t.Fatalf("expected %d transactions for debtor-pg, got %d", len(amounts), len(entityTx))
	}
	gotAmounts := make(map[float64]int)
	for _, e := range entityTx {
		gotAmounts[e.Amount]++
	}
	for _, want := range amounts {
		if gotAmounts[want] == 0 {
			t.Errorf("amount %v missing from GetTransactionsByEntity results (velocity CEL input); got %v", want, gotAmounts)
		}
		gotAmounts[want]--
	}

	// Clean up this run's rows so the shared scratch database stays small.
	_, _ = sqlRepo.db.ExecContext(ctx, sqlRepo.rebind(`
		DELETE FROM transactions WHERE tenant_id = ?`), tenant)
}
