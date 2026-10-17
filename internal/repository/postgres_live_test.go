package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// livePostgres opens a real PostgreSQL from OSPREY_POSTGRES_TEST_DSN, for
// example "host=localhost user=osprey dbname=osprey sslmode=disable". The test
// is skipped when the variable is not set, so the default unit run needs no
// server. Each test gets a fresh schema that is dropped at the end.
func livePostgres(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("OSPREY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set OSPREY_POSTGRES_TEST_DSN to run against a live PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	schema := fmt.Sprintf("osprey_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	if err != nil {
		t.Fatalf("open test connection: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		_ = admin.Close()
	})
	return db
}

func liveTransaction(id string, amount float64) *domain.Transaction {
	now := time.Now().UTC()
	return &domain.Transaction{ //nolint:gosec // G101: synthetic test identifiers, not credentials
		ID:              id,
		Type:            "transfer",
		DebtorID:        "debtor-pg",
		DebtorAccountID: "acc-pg-d",
		CreditorID:      "creditor-pg",
		CreditorAcctID:  "acc-pg-c",
		Amount:          amount,
		Currency:        "USD",
		Timestamp:       now,
		CreatedAt:       now,
	}
}

func columnType(t *testing.T, db *sql.DB, table, column string) string {
	t.Helper()
	var dataType string
	err := db.QueryRow(`SELECT data_type FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2`, table, column).Scan(&dataType)
	if err != nil {
		t.Fatalf("introspect %s.%s: %v", table, column, err)
	}
	return dataType
}

var realColumns = [][2]string{
	{"transactions", "amount"},
	{"rule_configs", "weight"},
	{"evaluations", "score"},
	{"typologies", "alert_threshold"},
}

// TestLivePostgresAmountRoundTrip is the end-to-end check for issue #65: on a
// fresh database an amount with more than 7 significant digits must come back
// unchanged.
func TestLivePostgresAmountRoundTrip(t *testing.T) {
	db := livePostgres(t)
	repo := &SQLRepository{db: db, driver: "postgres"}
	if err := repo.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, c := range realColumns {
		if got := columnType(t, db, c[0], c[1]); got != "double precision" {
			t.Errorf("%s.%s is %q, want double precision", c[0], c[1], got)
		}
	}

	ctx := context.Background()
	for i, amount := range []float64{1234567.89, 999999.99, 12345678.90, 0.1} {
		id := fmt.Sprintf("pg-amount-%d", i)
		if err := repo.SaveTransaction(ctx, "tenant-pg", liveTransaction(id, amount)); err != nil {
			t.Fatalf("SaveTransaction(%s): %v", id, err)
		}
		got, err := repo.GetTransaction(ctx, "tenant-pg", id)
		if err != nil {
			t.Fatalf("GetTransaction(%s): %v", id, err)
		}
		if got.Amount != amount {
			t.Errorf("amount round-trip: wrote %v, read %v", amount, got.Amount)
		}
	}
}

// TestLivePostgresUpgradesExistingREALColumns covers a database that an older
// build already created with float4 columns. CREATE TABLE IF NOT EXISTS leaves
// it as is, so migrate must alter the columns, and must keep the value the
// client saw (0.1, not 0.10000000149011612).
func TestLivePostgresUpgradesExistingREALColumns(t *testing.T) {
	db := livePostgres(t)

	// Create the tables the way the build before this fix did.
	for _, schema := range AllSchemas() {
		if _, err := db.Exec(strings.ReplaceAll(schema, " BLOB,", " BYTEA,")); err != nil {
			t.Fatalf("create old schema: %v", err)
		}
	}
	if got := columnType(t, db, "transactions", "amount"); got != "real" {
		t.Fatalf("precondition: transactions.amount is %q, want real", got)
	}
	repo := &SQLRepository{db: db, driver: "postgres"}
	ctx := context.Background()
	if err := repo.SaveTransaction(ctx, "tenant-pg", liveTransaction("old-1", 0.1)); err != nil {
		t.Fatalf("SaveTransaction: %v", err)
	}
	if err := repo.SaveTransaction(ctx, "tenant-pg", liveTransaction("old-2", 2500.5)); err != nil {
		t.Fatalf("SaveTransaction: %v", err)
	}

	if err := repo.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, c := range realColumns {
		if got := columnType(t, db, c[0], c[1]); got != "double precision" {
			t.Errorf("%s.%s is %q after migrate, want double precision", c[0], c[1], got)
		}
	}
	for id, want := range map[string]float64{"old-1": 0.1, "old-2": 2500.5} {
		got, err := repo.GetTransaction(ctx, "tenant-pg", id)
		if err != nil {
			t.Fatalf("GetTransaction(%s): %v", id, err)
		}
		if got.Amount != want {
			t.Errorf("%s: amount %v after upgrade, want %v", id, got.Amount, want)
		}
	}

	// A second start must be a no-op.
	if err := repo.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
