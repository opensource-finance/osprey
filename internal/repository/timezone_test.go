package repository

import (
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

// A transaction saved with a non-UTC offset must still fall inside both SQLite
// velocity windows. The windows compare stored times as text against UTC
// bounds, so a time stored as "+0200" sorted outside a window that holds it.
func TestSQLiteWindowsFindNonUTCTimestamps(t *testing.T) {
	s := newWindowSuite(t)
	defer func() { _ = s.repo.Close() }()

	plus2 := time.FixedZone("UTC+2", 2*60*60)
	event := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC).In(plus2) // 14:00+02:00
	created := time.Now().In(plus2)
	tx := &domain.Transaction{
		ID: "tz-1", Type: "transfer",
		DebtorID: "debtor-tz", DebtorAccountID: "acc-d",
		CreditorID: "creditor-tz", CreditorAcctID: "acc-c",
		Amount: 10, Currency: "USD",
		Timestamp: event, CreatedAt: created,
	}
	if err := s.repo.SaveTransaction(s.ctx, s.tenantID, tx); err != nil {
		t.Fatalf("SaveTransaction: %v", err)
	}

	from := time.Date(2026, 11, 15, 11, 59, 0, 0, time.UTC)
	to := time.Date(2026, 11, 15, 12, 1, 0, 0, time.UTC)
	got, err := s.repo.GetTransactionsByEntityInEventWindow(s.ctx, s.tenantID, "debtor-tz", from, to)
	if err != nil {
		t.Fatalf("event window: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("event window [11:59Z, 12:01Z]: want the 12:00Z transaction, got %d rows", len(got))
	}
	if n := len(s.fetch(t, "debtor-tz", -time.Hour)); n != 1 {
		t.Errorf("received window (last hour): want 1 row, got %d", n)
	}
}

// An evaluation saved with a non-UTC offset must read back. SQLite stored the
// zone name "UTC+2", which the driver cannot parse, so GetEvaluation failed
// with a scan error.
func TestSaveEvaluationNonUTCTimestamp(t *testing.T) {
	s := newWindowSuite(t)
	defer func() { _ = s.repo.Close() }()

	plus2 := time.FixedZone("UTC+2", 2*60*60)
	at := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC).In(plus2)
	eval := &domain.Evaluation{
		ID: "eval-tz-1", TxID: "tz-1", Status: domain.StatusNoAlert,
		Timestamp: at,
		Metadata:  domain.EvaluationMetadata{TraceID: "trace-tz"},
	}
	if err := s.repo.SaveEvaluation(s.ctx, s.tenantID, eval); err != nil {
		t.Fatalf("SaveEvaluation: %v", err)
	}
	got, err := s.repo.GetEvaluation(s.ctx, s.tenantID, eval.ID)
	if err != nil {
		t.Fatalf("GetEvaluation: %v", err)
	}
	if !got.Timestamp.Equal(at) {
		t.Errorf("timestamp: want %s, got %s", at.UTC(), got.Timestamp)
	}
}
