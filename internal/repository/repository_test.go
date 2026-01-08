package repository

import (
	"context"
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
