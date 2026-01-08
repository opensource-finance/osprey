// Package domain defines the core interfaces and types for Osprey.
package domain

import (
	"context"
	"time"
)

// Repository defines the interface for data persistence.
// All methods require tenantID for strict multi-tenancy isolation.
type Repository interface {
	// Transaction operations
	SaveTransaction(ctx context.Context, tenantID string, tx *Transaction) error
	GetTransaction(ctx context.Context, tenantID string, txID string) (*Transaction, error)
	GetTransactionsByEntity(ctx context.Context, tenantID string, entityID string, since time.Time) ([]*Transaction, error)

	// Rule configuration operations
	SaveRuleConfig(ctx context.Context, tenantID string, rule *RuleConfig) error
	GetRuleConfig(ctx context.Context, tenantID string, ruleID string) (*RuleConfig, error)
	ListRuleConfigs(ctx context.Context, tenantID string) ([]*RuleConfig, error)

	// Evaluation results
	SaveEvaluation(ctx context.Context, tenantID string, eval *Evaluation) error
	GetEvaluation(ctx context.Context, tenantID string, evalID string) (*Evaluation, error)

	// Typology configuration operations
	SaveTypology(ctx context.Context, tenantID string, typology *Typology) error
	GetTypology(ctx context.Context, tenantID string, typologyID string) (*Typology, error)
	ListTypologies(ctx context.Context, tenantID string) ([]*Typology, error)
	DeleteTypology(ctx context.Context, tenantID string, typologyID string) error

	// Health check
	Ping(ctx context.Context) error

	// Lifecycle
	Close() error
}

// RepositoryConfig holds configuration for repository initialization.
type RepositoryConfig struct {
	// Driver is the database driver: "sqlite" or "postgres"
	Driver string

	// SQLite specific
	SQLitePath string

	// PostgreSQL specific
	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string

	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}
