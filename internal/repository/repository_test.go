package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestSQLiteRepository(t *testing.T) {
	// Create temp database file
	tmpFile, err := os.CreateTemp("", "osprey-test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cfg := domain.RepositoryConfig{
		Driver:     "sqlite",
		SQLitePath: tmpPath,
	}

	repo, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("Ping", func(t *testing.T) {
		if err := repo.Ping(ctx); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	t.Run("SaveAndGetTransaction", func(t *testing.T) {
		tx := &domain.Transaction{
			ID:              "tx-001",
			Type:            "transfer",
			DebtorID:        "debtor-001",
			DebtorAccountID: "acc-001",
			CreditorID:      "creditor-001",
			CreditorAcctID:  "acc-002",
			Amount:          1000.00,
			Currency:        "USD",
			Timestamp:       time.Now().UTC(),
			CreatedAt:       time.Now().UTC(),
			Metadata:        map[string]any{"source": "api"},
		}

		if err := repo.SaveTransaction(ctx, tenantID, tx); err != nil {
			t.Fatalf("SaveTransaction failed: %v", err)
		}

		retrieved, err := repo.GetTransaction(ctx, tenantID, tx.ID)
		if err != nil {
			t.Fatalf("GetTransaction failed: %v", err)
		}

		if retrieved.ID != tx.ID {
			t.Errorf("expected ID %s, got %s", tx.ID, retrieved.ID)
		}
		if retrieved.Amount != tx.Amount {
			t.Errorf("expected Amount %.2f, got %.2f", tx.Amount, retrieved.Amount)
		}
		if retrieved.TenantID != tenantID {
			t.Errorf("expected TenantID %s, got %s", tenantID, retrieved.TenantID)
		}
	})

	t.Run("TenantIsolation", func(t *testing.T) {
		otherTenant := "tenant-002"

		// Try to get tx from different tenant
		_, err := repo.GetTransaction(ctx, otherTenant, "tx-001")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound for different tenant, got: %v", err)
		}
	})

	t.Run("AllowsSameTransactionIDAcrossTenants", func(t *testing.T) {
		tx := &domain.Transaction{
			ID:              "shared-client-id",
			Type:            "transfer",
			DebtorID:        "debtor-shared",
			DebtorAccountID: "acc-shared-1",
			CreditorID:      "creditor-shared",
			CreditorAcctID:  "acc-shared-2",
			Amount:          100,
			Currency:        "USD",
			Timestamp:       time.Now().UTC(),
			CreatedAt:       time.Now().UTC(),
		}

		if err := repo.SaveTransaction(ctx, "tenant-a", tx); err != nil {
			t.Fatalf("SaveTransaction tenant-a failed: %v", err)
		}
		if err := repo.SaveTransaction(ctx, "tenant-b", tx); err != nil {
			t.Fatalf("SaveTransaction tenant-b with same transaction ID failed: %v", err)
		}

		gotA, err := repo.GetTransaction(ctx, "tenant-a", tx.ID)
		if err != nil {
			t.Fatalf("GetTransaction tenant-a failed: %v", err)
		}
		gotB, err := repo.GetTransaction(ctx, "tenant-b", tx.ID)
		if err != nil {
			t.Fatalf("GetTransaction tenant-b failed: %v", err)
		}
		if gotA.TenantID != "tenant-a" || gotB.TenantID != "tenant-b" {
			t.Fatalf("expected tenant-scoped transaction IDs, got %q and %q", gotA.TenantID, gotB.TenantID)
		}
	})

	t.Run("DuplicateTransactionReturnsSentinel", func(t *testing.T) {
		tx := &domain.Transaction{
			ID:              "duplicate-client-id",
			Type:            "transfer",
			DebtorID:        "debtor-dup",
			DebtorAccountID: "acc-dup-1",
			CreditorID:      "creditor-dup",
			CreditorAcctID:  "acc-dup-2",
			Amount:          100,
			Currency:        "USD",
			Timestamp:       time.Now().UTC(),
			CreatedAt:       time.Now().UTC(),
		}

		if err := repo.SaveTransaction(ctx, tenantID, tx); err != nil {
			t.Fatalf("SaveTransaction failed: %v", err)
		}
		if err := repo.SaveTransaction(ctx, tenantID, tx); !errors.Is(err, ErrDuplicateTransaction) {
			t.Fatalf("expected ErrDuplicateTransaction, got %v", err)
		}
	})

	t.Run("RequiresTenantID", func(t *testing.T) {
		tx := &domain.Transaction{ID: "tx-test"}

		err := repo.SaveTransaction(ctx, "", tx)
		if err == nil {
			t.Error("expected error for empty tenantID")
		}

		_, err = repo.GetTransaction(ctx, "", "tx-001")
		if err == nil {
			t.Error("expected error for empty tenantID")
		}
	})

	t.Run("GetTransactionsByEntity", func(t *testing.T) {
		// Create another transaction
		tx2 := &domain.Transaction{
			ID:              "tx-002",
			Type:            "transfer",
			DebtorID:        "debtor-001", // Same debtor as tx-001
			DebtorAccountID: "acc-001",
			CreditorID:      "creditor-002",
			CreditorAcctID:  "acc-003",
			Amount:          500.00,
			Currency:        "USD",
			Timestamp:       time.Now().UTC(),
			CreatedAt:       time.Now().UTC(),
		}

		if err := repo.SaveTransaction(ctx, tenantID, tx2); err != nil {
			t.Fatalf("SaveTransaction failed: %v", err)
		}

		since := time.Now().Add(-1 * time.Hour)
		transactions, err := repo.GetTransactionsByEntity(ctx, tenantID, "debtor-001", since)
		if err != nil {
			t.Fatalf("GetTransactionsByEntity failed: %v", err)
		}

		if len(transactions) != 2 {
			t.Errorf("expected 2 transactions, got %d", len(transactions))
		}
	})

	t.Run("SaveAndGetEvaluation", func(t *testing.T) {
		eval := &domain.Evaluation{
			ID:        "eval-001",
			TxID:      "tx-001",
			Status:    domain.StatusNoAlert,
			Score:     0.15,
			Timestamp: time.Now().UTC(),
			RuleResults: []domain.RuleResult{
				{RuleID: "rule-001", Score: 0.1, SubRuleRef: domain.RuleOutcomePass},
			},
			Metadata: domain.EvaluationMetadata{TraceID: "trace-001"},
		}

		if err := repo.SaveEvaluation(ctx, tenantID, eval); err != nil {
			t.Fatalf("SaveEvaluation failed: %v", err)
		}

		retrieved, err := repo.GetEvaluation(ctx, tenantID, eval.ID)
		if err != nil {
			t.Fatalf("GetEvaluation failed: %v", err)
		}

		if retrieved.ID != eval.ID {
			t.Errorf("expected ID %s, got %s", eval.ID, retrieved.ID)
		}
		if retrieved.Score != eval.Score {
			t.Errorf("expected Score %.2f, got %.2f", eval.Score, retrieved.Score)
		}
		if retrieved.Status != eval.Status {
			t.Errorf("expected Status %s, got %s", eval.Status, retrieved.Status)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetTransaction(ctx, tenantID, "nonexistent")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}

		_, err = repo.GetEvaluation(ctx, tenantID, "nonexistent")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("CorruptRuleBandsReturnError", func(t *testing.T) {
		sqlRepo := repo.(*SQLRepository)
		_, err := sqlRepo.db.ExecContext(ctx, `
			INSERT INTO rule_configs (
				id, tenant_id, name, description, version, expression, bands, weight, enabled, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"corrupt-rule-bands",
			tenantID,
			"Corrupt Rule Bands",
			"",
			"1.0.0",
			"1 == 1",
			"not-json",
			1.0,
			1,
			time.Now().UTC(),
			time.Now().UTC(),
		)
		if err != nil {
			t.Fatalf("failed to insert corrupt rule config: %v", err)
		}

		if _, err := repo.GetRuleConfig(ctx, tenantID, "corrupt-rule-bands"); err == nil {
			t.Fatal("expected corrupt rule bands to fail on get")
		}
		if _, err := repo.ListRuleConfigs(ctx, tenantID); err == nil {
			t.Fatal("expected corrupt rule bands to fail on list")
		}
	})

	t.Run("RejectsUnencodableTransactionMetadata", func(t *testing.T) {
		tx := &domain.Transaction{
			ID:              "tx-bad-metadata",
			Type:            "transfer",
			DebtorID:        "debtor-bad",
			DebtorAccountID: "account-bad",
			CreditorID:      "creditor-bad",
			CreditorAcctID:  "account-bad-2",
			Amount:          100,
			Currency:        "USD",
			Timestamp:       time.Now().UTC(),
			CreatedAt:       time.Now().UTC(),
			Metadata:        map[string]any{"bad": func() {}},
		}

		if err := repo.SaveTransaction(ctx, tenantID, tx); err == nil {
			t.Fatal("expected unencodable transaction metadata to fail")
		}
	})

	t.Run("CorruptTransactionMetadataReturnsError", func(t *testing.T) {
		sqlRepo := repo.(*SQLRepository)
		_, err := sqlRepo.db.ExecContext(ctx, `
			INSERT INTO transactions (
				id, tenant_id, type, debtor_id, debtor_account_id,
				creditor_id, creditor_account_id, amount, currency,
				timestamp, created_at, metadata, original_message
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"tx-corrupt-metadata",
			tenantID,
			"transfer",
			"debtor-corrupt",
			"account-corrupt",
			"creditor-corrupt",
			"account-corrupt-2",
			100,
			"USD",
			time.Now().UTC(),
			time.Now().UTC(),
			"not-json",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to insert corrupt transaction: %v", err)
		}

		if _, err := repo.GetTransaction(ctx, tenantID, "tx-corrupt-metadata"); err == nil {
			t.Fatal("expected corrupt transaction metadata to fail on get")
		}
		if _, err := repo.GetTransactionsByEntity(ctx, tenantID, "debtor-corrupt", time.Now().Add(-time.Hour)); err == nil {
			t.Fatal("expected corrupt transaction metadata to fail on list")
		}
	})

	t.Run("CorruptEvaluationJSONReturnsError", func(t *testing.T) {
		sqlRepo := repo.(*SQLRepository)
		_, err := sqlRepo.db.ExecContext(ctx, `
			INSERT INTO evaluations (
				id, tenant_id, tx_id, status, score, timestamp,
				rule_results, typology_results, metadata
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"eval-corrupt-json",
			tenantID,
			"tx-corrupt-metadata",
			domain.StatusNoAlert,
			0,
			time.Now().UTC(),
			"not-json",
			"[]",
			"{}",
		)
		if err != nil {
			t.Fatalf("failed to insert corrupt evaluation: %v", err)
		}

		if _, err := repo.GetEvaluation(ctx, tenantID, "eval-corrupt-json"); err == nil {
			t.Fatal("expected corrupt evaluation JSON to fail on get")
		}
	})
}

func TestSQLiteMigratesLegacyTransactionPrimaryKey(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "osprey-legacy-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + "-shm")
	defer os.Remove(tmpPath + "-wal")

	db, err := sql.Open("sqlite", "file:"+tmpPath)
	if err != nil {
		t.Fatalf("failed to open legacy database: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE transactions (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			type TEXT NOT NULL,
			debtor_id TEXT NOT NULL,
			debtor_account_id TEXT NOT NULL,
			creditor_id TEXT NOT NULL,
			creditor_account_id TEXT NOT NULL,
			amount REAL NOT NULL,
			currency TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL,
			metadata TEXT,
			original_message BLOB
		);
		INSERT INTO transactions (
			id, tenant_id, type, debtor_id, debtor_account_id,
			creditor_id, creditor_account_id, amount, currency,
			timestamp, created_at, metadata, original_message
		) VALUES (
			'client-tx-001', 'tenant-a', 'TRANSFER', 'debtor-a', 'account-a',
			'creditor-a', 'account-b', 100, 'USD',
			'2026-05-25T10:00:00Z', '2026-05-25T10:00:00Z', '{}', NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to create legacy schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close legacy database: %v", err)
	}

	repo, err := New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: tmpPath})
	if err != nil {
		t.Fatalf("failed to open migrated repository: %v", err)
	}
	defer repo.Close()

	sqlRepo := repo.(*SQLRepository)
	rows, err := sqlRepo.db.Query(`PRAGMA table_info(transactions)`)
	if err != nil {
		t.Fatalf("failed to inspect migrated schema: %v", err)
	}
	defer rows.Close()

	primaryKey := make(map[string]int)
	for rows.Next() {
		var (
			cid       int
			name      string
			columnTyp string
			notNull   int
			defaultV  sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &columnTyp, &notNull, &defaultV, &pk); err != nil {
			t.Fatalf("failed to scan table info: %v", err)
		}
		primaryKey[name] = pk
	}
	if primaryKey["id"] == 0 || primaryKey["tenant_id"] == 0 {
		t.Fatalf("expected composite transaction primary key, got id=%d tenant_id=%d", primaryKey["id"], primaryKey["tenant_id"])
	}

	ctx := context.Background()
	got, err := repo.GetTransaction(ctx, "tenant-a", "client-tx-001")
	if err != nil {
		t.Fatalf("expected legacy transaction to survive migration: %v", err)
	}
	if got.ID != "client-tx-001" || got.TenantID != "tenant-a" {
		t.Fatalf("unexpected migrated transaction: %#v", got)
	}

	tx := &domain.Transaction{
		ID:              "client-tx-001",
		Type:            "TRANSFER",
		DebtorID:        "debtor-b",
		DebtorAccountID: "account-c",
		CreditorID:      "creditor-b",
		CreditorAcctID:  "account-d",
		Amount:          200,
		Currency:        "USD",
		Timestamp:       time.Now().UTC(),
		CreatedAt:       time.Now().UTC(),
	}
	if err := repo.SaveTransaction(ctx, "tenant-b", tx); err != nil {
		t.Fatalf("expected same transaction id in another tenant after migration: %v", err)
	}
}

func TestUnsupportedDriver(t *testing.T) {
	cfg := domain.RepositoryConfig{
		Driver: "mysql",
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for unsupported driver")
	}
}

func TestRebind(t *testing.T) {
	repo := &SQLRepository{driver: "postgres"}

	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT * FROM t WHERE id = ?", "SELECT * FROM t WHERE id = $1"},
		{"INSERT INTO t (a, b) VALUES (?, ?)", "INSERT INTO t (a, b) VALUES ($1, $2)"},
		{"SELECT * FROM t", "SELECT * FROM t"},
	}

	for _, tt := range tests {
		result := repo.rebind(tt.input)
		if result != tt.expected {
			t.Errorf("rebind(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
