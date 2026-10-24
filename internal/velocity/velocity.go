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

// GetTransactionCount returns the number of transactions for an entity within a time window.
// This is the VelocityGetter function signature expected by the rule engine.
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
// Count and AmountSum cover every transaction where the entity is the debtor or
// the creditor. DistinctCreditors counts payees of outgoing transactions only.
// The Inbound fields cover transactions where the entity is only the creditor.
type Aggregates struct {
	Count             int64
	AmountSum         float64
	DistinctCreditors int64
	InboundCount      int64
	InboundAmountSum  float64
}

// GetAggregates returns velocity metrics for an entity in a window that ends now,
// keyed on the time Osprey received each transaction (the "received" clock).
//
// currency scopes the monetary sums (AmountSum, InboundAmountSum) to transactions
// in that currency so a window that mixes currencies does not add incomparable
// amounts. The dimensionless counters (Count, DistinctCreditors, InboundCount)
// stay currency-agnostic. A empty currency sums all rows (legacy fallback).
func (s *Service) GetAggregates(ctx context.Context, tenantID, entityID, currency string, windowSecs int) (Aggregates, error) {
	if tenantID == "" || entityID == "" {
		return Aggregates{}, fmt.Errorf("tenantID and entityID are required")
	}
	if s.repo == nil {
		return Aggregates{}, fmt.Errorf("no data source available")
	}
	txs, err := s.repo.GetTransactionsByEntity(ctx, tenantID, entityID, windowStart(windowSecs))
	if err != nil {
		return Aggregates{}, fmt.Errorf("failed to get transactions: %w", err)
	}
	return aggregate(entityID, currency, txs), nil
}

// GetAggregatesAt returns velocity metrics for an entity in the window that ends at
// the event time at, keyed on each transaction's own timestamp (the "event" clock).
// Replays of historical data need this clock: with the received clock, a replay
// puts months of activity into a few minutes.
//
// currency scopes the monetary sums (AmountSum, InboundAmountSum) to transactions
// in that currency so a window that mixes currencies does not add incomparable
// amounts. The dimensionless counters (Count, DistinctCreditors, InboundCount)
// stay currency-agnostic. A empty currency sums all rows (legacy fallback).
func (s *Service) GetAggregatesAt(ctx context.Context, tenantID, entityID, currency string, windowSecs int, at time.Time) (Aggregates, error) {
	if tenantID == "" || entityID == "" {
		return Aggregates{}, fmt.Errorf("tenantID and entityID are required")
	}
	if s.repo == nil {
		return Aggregates{}, fmt.Errorf("no data source available")
	}
	windowSecs = min(windowSecs, MaxWindowSecs)
	from := at.Add(-time.Duration(windowSecs) * time.Second)
	txs, err := s.repo.GetTransactionsByEntityInEventWindow(ctx, tenantID, entityID, from, at)
	if err != nil {
		return Aggregates{}, fmt.Errorf("failed to get transactions: %w", err)
	}
	return aggregate(entityID, currency, txs), nil
}

// aggregate computes the metrics in Go from the repository result set.
//
// The dimensionless counters (Count, DistinctCreditors, InboundCount) cover every
// transaction in the window regardless of currency. The monetary sums
// (AmountSum, InboundAmountSum) only add transactions whose Currency matches the
// evaluated transaction's currency, so a multi-currency window never produces a
// currency-incoherent total. A empty currency matches every row (legacy fallback).
func aggregate(entityID, currency string, txs []*domain.Transaction) Aggregates {
	a := Aggregates{Count: int64(len(txs))}
	creditors := make(map[string]struct{}, len(txs))
	for _, tx := range txs {
		if currency == "" || tx.Currency == currency {
			a.AmountSum += tx.Amount
		}
		if tx.DebtorID == entityID {
			if tx.CreditorID != "" {
				creditors[tx.CreditorID] = struct{}{}
			}
			continue
		}
		a.InboundCount++
		if currency == "" || tx.Currency == currency {
			a.InboundAmountSum += tx.Amount
		}
	}
	a.DistinctCreditors = int64(len(creditors))
	return a
}
