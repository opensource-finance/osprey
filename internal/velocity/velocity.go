// Package velocity provides transaction velocity calculation.
package velocity

import (
	"context"
	"fmt"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// MaxWindowSecs is the longest velocity window, 30 days. A larger window is
// clamped to it. Without the clamp, a window above about 9.2e9 seconds
// overflows time.Duration, wraps negative, and puts the window start in the
// future, so every velocity metric reads 0 and velocity rules never fire.
const MaxWindowSecs = 30 * 24 * 60 * 60

// windowStart returns the start of a velocity window that ends now.
func windowStart(windowSecs int) time.Time {
	windowSecs = min(windowSecs, MaxWindowSecs)
	return time.Now().UTC().Add(-time.Duration(windowSecs) * time.Second)
}

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

// GetTransactionCount returns the number of outgoing transactions sent by an
// entity within a time window. Velocity is a debtor-side metric (see the
// velocity_* variable catalog: "transactions from the debtor"), so only rows
// where the entity is the debtor count. This is the VelocityGetter function
// signature expected by the rule engine.
func (s *Service) GetTransactionCount(ctx context.Context, tenantID, entityID string, windowSecs int) (int64, error) {
	if tenantID == "" || entityID == "" {
		return 0, fmt.Errorf("tenantID and entityID are required")
	}

	// Query the repository for the actual count (caching would require careful TTL management)
	since := windowStart(windowSecs)

	if s.repo != nil {
		return s.countFromRepo(ctx, tenantID, entityID, since)
	}

	return 0, fmt.Errorf("no data source available")
}

// countFromRepo uses the repository to get outgoing transactions and count them.
func (s *Service) countFromRepo(ctx context.Context, tenantID, entityID string, since time.Time) (int64, error) {
	txs, err := s.outgoingTransactions(ctx, tenantID, entityID, since)
	if err != nil {
		return 0, err
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

	since := windowStart(windowSecs)

	if s.repo != nil {
		return s.aggregatesFromRepo(ctx, tenantID, entityID, since)
	}
	return Aggregates{}, fmt.Errorf("no data source available")
}

// aggregatesFromRepo computes the metrics in Go from the repository result set.
func (s *Service) aggregatesFromRepo(ctx context.Context, tenantID, entityID string, since time.Time) (Aggregates, error) {
	txs, err := s.outgoingTransactions(ctx, tenantID, entityID, since)
	if err != nil {
		return Aggregates{}, err
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

// outgoingTransactions returns the rows from GetTransactionsByEntity in which
// entityID is the debtor. GetTransactionsByEntity matches the entity in either
// role (debtor_id = ? OR creditor_id = ?) because other callers want both
// directions, but the velocity_* metrics are defined for the debtor only.
// Counting incoming rows inflated velocity_count and velocity_amount_sum, and
// for incoming rows CreditorID is the entity itself, which surfaced as the
// entity counting as its own creditor in velocity_distinct_creditors.
func (s *Service) outgoingTransactions(ctx context.Context, tenantID, entityID string, since time.Time) ([]*domain.Transaction, error) {
	txs, err := s.repo.GetTransactionsByEntity(ctx, tenantID, entityID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	out := make([]*domain.Transaction, 0, len(txs))
	for _, tx := range txs {
		if tx.DebtorID != entityID {
			continue
		}
		out = append(out, tx)
	}
	return out, nil
}
