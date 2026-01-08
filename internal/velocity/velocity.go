// Package velocity provides transaction velocity calculation.
package velocity

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// Service calculates transaction velocity for entities.
type Service struct {
	repo  domain.Repository
	cache domain.Cache
	db    *sql.DB // Direct DB access for custom queries
}

// NewService creates a new velocity service.
func NewService(repo domain.Repository, cache domain.Cache) *Service {
	return &Service{
		repo:  repo,
		cache: cache,
	}
}


// GetTransactionCount returns the number of transactions for an entity within a time window.
// This is the VelocityGetter function signature expected by the rule engine.
func (s *Service) GetTransactionCount(ctx context.Context, tenantID, entityID string, windowSecs int) (int64, error) {
	if tenantID == "" || entityID == "" {
		return 0, fmt.Errorf("tenantID and entityID are required")
	}

	// Query database for actual count (caching would require careful TTL management)
	since := time.Now().Add(-time.Duration(windowSecs) * time.Second)

	if s.db != nil {
		return s.countFromDB(ctx, tenantID, entityID, since)
	}

	if s.repo != nil {
		return s.countFromRepo(ctx, tenantID, entityID, since)
	}

	return 0, fmt.Errorf("no data source available")
}

// countFromDB queries the database directly for transaction count.
func (s *Service) countFromDB(ctx context.Context, tenantID, entityID string, since time.Time) (int64, error) {
	query := `
		SELECT COUNT(*) FROM transactions
		WHERE tenant_id = ?
		AND (debtor_id = ? OR creditor_id = ?)
		AND timestamp >= ?
	`

	var count int64
	err := s.db.QueryRowContext(ctx, query, tenantID, entityID, entityID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	return count, nil
}

// countFromRepo uses the repository to get transactions and count them.
func (s *Service) countFromRepo(ctx context.Context, tenantID, entityID string, since time.Time) (int64, error) {
	txs, err := s.repo.GetTransactionsByEntity(ctx, tenantID, entityID, since)
	if err != nil {
		return 0, fmt.Errorf("failed to get transactions: %w", err)
	}
	return int64(len(txs)), nil
}

// GetVelocityGetter returns a VelocityGetter function for the rule engine.
func (s *Service) GetVelocityGetter() func(ctx context.Context, tenantID, entityID string, windowSecs int) (int64, error) {
	return s.GetTransactionCount
}
