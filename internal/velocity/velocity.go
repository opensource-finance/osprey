// Package velocity provides transaction velocity calculation.
package velocity

import (
	"context"
	"fmt"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// Service calculates transaction velocity for entities.
type Service struct {
	repo  domain.Repository
	cache domain.Cache
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

	// Query the repository for the actual count (caching would require careful TTL management)
	since := time.Now().UTC().Add(-time.Duration(windowSecs) * time.Second)

	if s.repo != nil {
		return s.countFromRepo(ctx, tenantID, entityID, since)
	}

	return 0, fmt.Errorf("no data source available")
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

// Aggregates holds windowed velocity metrics for an entity.
type Aggregates struct {
	Count             int64
	AmountSum         float64
	DistinctCreditors int64
}

// GetAggregates returns count, amount-sum, and distinct-creditor metrics for an
// entity within a time window. Richer companion to GetTransactionCount.
func (s *Service) GetAggregates(ctx context.Context, tenantID, entityID string, windowSecs int) (Aggregates, error) {
	if tenantID == "" || entityID == "" {
		return Aggregates{}, fmt.Errorf("tenantID and entityID are required")
	}

	since := time.Now().UTC().Add(-time.Duration(windowSecs) * time.Second)

	if s.repo != nil {
		return s.aggregatesFromRepo(ctx, tenantID, entityID, since)
	}
	return Aggregates{}, fmt.Errorf("no data source available")
}

// aggregatesFromRepo computes the metrics in Go from the repository result set.
func (s *Service) aggregatesFromRepo(ctx context.Context, tenantID, entityID string, since time.Time) (Aggregates, error) {
	txs, err := s.repo.GetTransactionsByEntity(ctx, tenantID, entityID, since)
	if err != nil {
		return Aggregates{}, fmt.Errorf("failed to get transactions: %w", err)
	}
	a := Aggregates{Count: int64(len(txs))}
	creditors := make(map[string]struct{}, len(txs))
	for _, tx := range txs {
		a.AmountSum += tx.Amount
		if tx.CreditorID != "" {
			creditors[tx.CreditorID] = struct{}{}
		}
	}
	a.DistinctCreditors = int64(len(creditors))
	return a, nil
}
