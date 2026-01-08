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

	// Optional metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Reference to original message (for ISO 20022 adapter)
	OriginalMessage []byte `json:"-"`
}

// TransactionRequest is the API request payload for transaction evaluation.
type TransactionRequest struct {
	TenantID string                 `json:"tenantId" validate:"required"`
	Type     string                 `json:"type" validate:"required"`
	Debtor   Party                  `json:"debtor" validate:"required"`
	Creditor Party                  `json:"creditor" validate:"required"`
	Amount   Amount                 `json:"amount" validate:"required"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Party represents a transaction participant.
type Party struct {
	ID        string `json:"id" validate:"required"`
	AccountID string `json:"accountId" validate:"required"`
	Name      string `json:"name,omitempty"`
	Country   string `json:"country,omitempty"`
}

// Amount represents a monetary value.
type Amount struct {
	Value    float64 `json:"value" validate:"required,gt=0"`
	Currency string  `json:"currency" validate:"required,len=3"`
}

// ToTransaction converts a request to a Transaction domain object.
func (r *TransactionRequest) ToTransaction() *Transaction {
	now := time.Now().UTC()
	return &Transaction{
		TenantID:        r.TenantID,
		Type:            r.Type,
		DebtorID:        r.Debtor.ID,
		DebtorAccountID: r.Debtor.AccountID,
		CreditorID:      r.Creditor.ID,
		CreditorAcctID:  r.Creditor.AccountID,
		Amount:          r.Amount.Value,
		Currency:        r.Amount.Currency,
		Timestamp:       now,
		CreatedAt:       now,
		Metadata:        r.Metadata,
	}
}
