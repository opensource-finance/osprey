// Package repository provides data persistence implementations.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

var (
	ErrNotFound     = errors.New("record not found")
	ErrInvalidInput = errors.New("invalid input")
)

// SQLRepository implements domain.Repository using database/sql.
// Works with both SQLite and PostgreSQL drivers.
type SQLRepository struct {
	db     *sql.DB
	driver string
}

// New creates a new repository based on configuration.
func New(cfg domain.RepositoryConfig) (domain.Repository, error) {
	var db *sql.DB
	var err error

	switch cfg.Driver {
	case "sqlite":
		db, err = openSQLite(cfg)
	case "postgres":
		db, err = openPostgres(cfg)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	repo := &SQLRepository{
		db:     db,
		driver: cfg.Driver,
	}

	// Run migrations
	if err := repo.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return repo, nil
}

func (r *SQLRepository) migrate() error {
	for _, schema := range AllSchemas() {
		if _, err := r.db.Exec(schema); err != nil {
			return err
		}
	}
	return nil
}

// SaveTransaction stores a transaction with tenant isolation.
func (r *SQLRepository) SaveTransaction(ctx context.Context, tenantID string, tx *domain.Transaction) error {
	if tenantID == "" {
		return fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	metadata, _ := json.Marshal(tx.Metadata)

	query := `
		INSERT INTO transactions (
			id, tenant_id, type, debtor_id, debtor_account_id,
			creditor_id, creditor_account_id, amount, currency,
			timestamp, created_at, metadata, original_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, r.rebind(query),
		tx.ID, tenantID, tx.Type,
		tx.DebtorID, tx.DebtorAccountID,
		tx.CreditorID, tx.CreditorAcctID,
		tx.Amount, tx.Currency,
		tx.Timestamp, tx.CreatedAt,
		string(metadata), tx.OriginalMessage,
	)
	return err
}

// GetTransaction retrieves a transaction by ID with tenant isolation.
func (r *SQLRepository) GetTransaction(ctx context.Context, tenantID string, txID string) (*domain.Transaction, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	query := `
		SELECT id, tenant_id, type, debtor_id, debtor_account_id,
			   creditor_id, creditor_account_id, amount, currency,
			   timestamp, created_at, metadata
		FROM transactions
		WHERE tenant_id = ? AND id = ?
	`

	var tx domain.Transaction
	var metadata string

	err := r.db.QueryRowContext(ctx, r.rebind(query), tenantID, txID).Scan(
		&tx.ID, &tx.TenantID, &tx.Type,
		&tx.DebtorID, &tx.DebtorAccountID,
		&tx.CreditorID, &tx.CreditorAcctID,
		&tx.Amount, &tx.Currency,
		&tx.Timestamp, &tx.CreatedAt,
		&metadata,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if metadata != "" {
		json.Unmarshal([]byte(metadata), &tx.Metadata)
	}

	return &tx, nil
}

// GetTransactionsByEntity retrieves transactions for an entity with tenant isolation.
func (r *SQLRepository) GetTransactionsByEntity(ctx context.Context, tenantID string, entityID string, since time.Time) ([]*domain.Transaction, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	query := `
		SELECT id, tenant_id, type, debtor_id, debtor_account_id,
			   creditor_id, creditor_account_id, amount, currency,
			   timestamp, created_at, metadata
		FROM transactions
		WHERE tenant_id = ?
		  AND (debtor_id = ? OR creditor_id = ?)
		  AND timestamp >= ?
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, r.rebind(query), tenantID, entityID, entityID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*domain.Transaction
	for rows.Next() {
		var tx domain.Transaction
		var metadata string

		if err := rows.Scan(
			&tx.ID, &tx.TenantID, &tx.Type,
			&tx.DebtorID, &tx.DebtorAccountID,
			&tx.CreditorID, &tx.CreditorAcctID,
			&tx.Amount, &tx.Currency,
			&tx.Timestamp, &tx.CreatedAt,
			&metadata,
		); err != nil {
			return nil, err
		}

		if metadata != "" {
			json.Unmarshal([]byte(metadata), &tx.Metadata)
		}

		transactions = append(transactions, &tx)
	}

	return transactions, rows.Err()
}

// SaveRuleConfig stores a rule configuration with tenant isolation.
func (r *SQLRepository) SaveRuleConfig(ctx context.Context, tenantID string, rule *domain.RuleConfig) error {
	if tenantID == "" {
		return fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	bands, _ := json.Marshal(rule.Bands)

	enabled := 0
	if rule.Enabled {
		enabled = 1
	}

	now := time.Now().UTC()

	query := `
		INSERT INTO rule_configs (
			id, tenant_id, name, description, version, expression, bands, weight, enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, tenant_id, version) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			expression = excluded.expression,
			bands = excluded.bands,
			weight = excluded.weight,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`

	_, err := r.db.ExecContext(ctx, r.rebind(query),
		rule.ID, tenantID, rule.Name, rule.Description,
		rule.Version, rule.Expression, string(bands), rule.Weight, enabled,
		now, now,
	)
	return err
}

// GetRuleConfig retrieves a rule configuration with tenant isolation.
func (r *SQLRepository) GetRuleConfig(ctx context.Context, tenantID string, ruleID string) (*domain.RuleConfig, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	query := `
		SELECT id, tenant_id, name, description, version, expression, bands, weight, enabled
		FROM rule_configs
		WHERE tenant_id = ? AND id = ? AND enabled = 1
		ORDER BY version DESC
		LIMIT 1
	`

	var cfg domain.RuleConfig
	var bands string
	var enabled int

	err := r.db.QueryRowContext(ctx, r.rebind(query), tenantID, ruleID).Scan(
		&cfg.ID, &cfg.TenantID, &cfg.Name, &cfg.Description,
		&cfg.Version, &cfg.Expression, &bands, &cfg.Weight, &enabled,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	cfg.Enabled = enabled == 1
	json.Unmarshal([]byte(bands), &cfg.Bands)

	return &cfg, nil
}

// ListRuleConfigs retrieves all active rule configurations for a tenant.
func (r *SQLRepository) ListRuleConfigs(ctx context.Context, tenantID string) ([]*domain.RuleConfig, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	query := `
		SELECT id, tenant_id, name, description, version, expression, bands, weight, enabled
		FROM rule_configs
		WHERE tenant_id = ? AND enabled = 1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, r.rebind(query), tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*domain.RuleConfig
	for rows.Next() {
		var cfg domain.RuleConfig
		var bands string
		var enabled int

		if err := rows.Scan(
			&cfg.ID, &cfg.TenantID, &cfg.Name, &cfg.Description,
			&cfg.Version, &cfg.Expression, &bands, &cfg.Weight, &enabled,
		); err != nil {
			return nil, err
		}

		cfg.Enabled = enabled == 1
		json.Unmarshal([]byte(bands), &cfg.Bands)
		configs = append(configs, &cfg)
	}

	return configs, rows.Err()
}

// SaveEvaluation stores an evaluation result with tenant isolation.
func (r *SQLRepository) SaveEvaluation(ctx context.Context, tenantID string, eval *domain.Evaluation) error {
	if tenantID == "" {
		return fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	ruleResults, _ := json.Marshal(eval.RuleResults)
	typologyResults, _ := json.Marshal(eval.TypologyResults)
	metadata, _ := json.Marshal(eval.Metadata)

	query := `
		INSERT INTO evaluations (
			id, tenant_id, tx_id, status, score, timestamp,
			rule_results, typology_results, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, r.rebind(query),
		eval.ID, tenantID, eval.TxID, eval.Status, eval.Score, eval.Timestamp,
		string(ruleResults), string(typologyResults), string(metadata),
	)
	return err
}

// GetEvaluation retrieves an evaluation by ID with tenant isolation.
func (r *SQLRepository) GetEvaluation(ctx context.Context, tenantID string, evalID string) (*domain.Evaluation, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	query := `
		SELECT id, tenant_id, tx_id, status, score, timestamp,
			   rule_results, typology_results, metadata
		FROM evaluations
		WHERE tenant_id = ? AND id = ?
	`

	var eval domain.Evaluation
	var ruleResults, typologyResults, metadata string

	err := r.db.QueryRowContext(ctx, r.rebind(query), tenantID, evalID).Scan(
		&eval.ID, &eval.TenantID, &eval.TxID, &eval.Status, &eval.Score, &eval.Timestamp,
		&ruleResults, &typologyResults, &metadata,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(ruleResults), &eval.RuleResults)
	json.Unmarshal([]byte(typologyResults), &eval.TypologyResults)
	json.Unmarshal([]byte(metadata), &eval.Metadata)

	return &eval, nil
}

// SaveTypology stores a typology configuration with tenant isolation.
func (r *SQLRepository) SaveTypology(ctx context.Context, tenantID string, typology *domain.Typology) error {
	if tenantID == "" {
		return fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	rules, _ := json.Marshal(typology.Rules)

	enabled := 0
	if typology.Enabled {
		enabled = 1
	}

	now := time.Now().UTC()

	query := `
		INSERT INTO typologies (
			id, tenant_id, name, description, version, rules, alert_threshold, enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, tenant_id, version) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			rules = excluded.rules,
			alert_threshold = excluded.alert_threshold,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`

	_, err := r.db.ExecContext(ctx, r.rebind(query),
		typology.ID, tenantID, typology.Name, typology.Description,
		typology.Version, string(rules), typology.AlertThreshold, enabled,
		now, now,
	)
	return err
}

// GetTypology retrieves a typology configuration with tenant isolation.
func (r *SQLRepository) GetTypology(ctx context.Context, tenantID string, typologyID string) (*domain.Typology, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	query := `
		SELECT id, tenant_id, name, description, version, rules, alert_threshold, enabled, created_at, updated_at
		FROM typologies
		WHERE tenant_id = ? AND id = ? AND enabled = 1
		ORDER BY version DESC
		LIMIT 1
	`

	var t domain.Typology
	var rules string
	var enabled int

	err := r.db.QueryRowContext(ctx, r.rebind(query), tenantID, typologyID).Scan(
		&t.ID, &t.TenantID, &t.Name, &t.Description,
		&t.Version, &rules, &t.AlertThreshold, &enabled,
		&t.CreatedAt, &t.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	t.Enabled = enabled == 1
	if err := json.Unmarshal([]byte(rules), &t.Rules); err != nil {
		return nil, fmt.Errorf("failed to parse typology rules: %w", err)
	}

	return &t, nil
}

// ListTypologies retrieves all active typology configurations for a tenant.
func (r *SQLRepository) ListTypologies(ctx context.Context, tenantID string) ([]*domain.Typology, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	query := `
		SELECT id, tenant_id, name, description, version, rules, alert_threshold, enabled, created_at, updated_at
		FROM typologies
		WHERE tenant_id = ? AND enabled = 1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, r.rebind(query), tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var typologies []*domain.Typology
	for rows.Next() {
		var t domain.Typology
		var rules string
		var enabled int

		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.Name, &t.Description,
			&t.Version, &rules, &t.AlertThreshold, &enabled,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}

		t.Enabled = enabled == 1
		if err := json.Unmarshal([]byte(rules), &t.Rules); err != nil {
			return nil, fmt.Errorf("failed to parse typology rules for %s: %w", t.ID, err)
		}
		typologies = append(typologies, &t)
	}

	return typologies, rows.Err()
}

// DeleteTypology soft-deletes a typology by setting enabled = 0.
func (r *SQLRepository) DeleteTypology(ctx context.Context, tenantID string, typologyID string) error {
	if tenantID == "" {
		return fmt.Errorf("%w: tenantID is required", ErrInvalidInput)
	}

	query := `
		UPDATE typologies
		SET enabled = 0, updated_at = ?
		WHERE tenant_id = ? AND id = ?
	`

	result, err := r.db.ExecContext(ctx, r.rebind(query), time.Now().UTC(), tenantID, typologyID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// Ping checks database connectivity.
func (r *SQLRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close closes the database connection.
func (r *SQLRepository) Close() error {
	return r.db.Close()
}

// rebind converts ? placeholders to $1, $2, etc. for PostgreSQL.
func (r *SQLRepository) rebind(query string) string {
	if r.driver != "postgres" {
		return query
	}

	// Convert ? to $1, $2, etc.
	var result []byte
	n := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			result = append(result, '$')
			result = append(result, fmt.Sprintf("%d", n)...)
			n++
		} else {
			result = append(result, query[i])
		}
	}
	return string(result)
}
