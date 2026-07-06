package domain

import (
	"time"
)

// Transaction represents an incoming transaction to be evaluated.
type Transaction struct {
	// Core identifiers
	ID       string `json:"id"`
	TenantID string `json:"tenantId"`

	// Transaction type (e.g., "transfer", "payment", "withdrawal")
	Type string `json:"type"`

	// Parties involved
	DebtorID        string `json:"debtorId"`
	DebtorAccountID string `json:"debtorAccountId"`
	CreditorID      string `json:"creditorId"`
	CreditorAcctID  string `json:"creditorAccountId"`

	// Financial details
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`

	// Temporal
	Timestamp time.Time `json:"timestamp"`
	CreatedAt time.Time `json:"createdAt"`

	// Optional metadata (caller-asserted facts; exposed to rules as the `meta` map)
	Metadata map[string]any `json:"metadata,omitempty"`

	// Optional enrichment (externally-computed scores/flags; exposed as the `enrichment` map)
	Enrichment map[string]any `json:"enrichment,omitempty"`

	// Reference to original message (for ISO 20022 adapter)
	OriginalMessage []byte `json:"-"`
}
