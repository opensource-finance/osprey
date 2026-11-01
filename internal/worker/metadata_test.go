package worker

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
)

// drainRule is the account-drain rule shape shipped in configs/rules/fatf-rules.json
// and paysim-rules.json. It reads metadata-only balance variables. Before the fix,
// the worker dropped the metadata key on unmarshal, so both balances defaulted to
// 0.0 and this rule never fired on the async path.
func drainRule() []*domain.RuleConfig {
	zero, one := 0.0, 1.0
	return []*domain.RuleConfig{
		{
			ID:         "account-drain-001",
			Name:       "Account Drain",
			Expression: "old_balance > 0.0 && new_balance == 0.0",
			Bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomePass, Reason: "no drain"},
				{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeReview, Reason: "account drained"},
			},
			Weight:  1.0,
			Enabled: true,
		},
	}
}

// metaReadRule reads an unguarded meta.* key. Before the fix, the meta map was
// empty on the async path when the producer used the documented metadata key, so
// this rule errored with "no such key" and the evaluation flipped to ALRT with
// score 0.0 from the fail-closed error path instead of the real signal.
func metaReadRule() []*domain.RuleConfig {
	zero, one := 0.0, 1.0
	return []*domain.RuleConfig{
		{
			ID:         "meta-country-001",
			Name:       "Meta Country",
			Expression: `meta.country == "US"`,
			Bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomePass, Reason: "not US"},
				{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeReview, Reason: "US transaction"},
			},
			Weight:  1.0,
			Enabled: true,
		},
	}
}

// drainPayload builds a JSON TransactionMessage payload with the given metadata
// under the requested top-level key. The producer's only knob is the key name,
// which is the crux of the bug.
func drainPayload(t *testing.T, key string, metadata map[string]any) []byte {
	t.Helper()
	raw := map[string]any{
		"txId":       "tx-drain",
		"tenantId":   "tenantA",
		"traceId":    "trace-drain",
		"type":       "transfer",
		"debtorId":   "d1",
		"creditorId": "c1",
		"amount":     15000.0,
		"currency":   "USD",
		key:          metadata,
	}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal drain payload: %v", err)
	}
	return b
}

// payloadWithBoth builds a JSON payload that carries both the documented metadata
// key and the legacy additionalData alias. The fix must prefer metadata.
func payloadWithBoth(t *testing.T, metadata, additionalData map[string]any) []byte {
	t.Helper()
	raw := map[string]any{
		"txId":           "tx-both",
		"tenantId":       "tenantA",
		"traceId":        "trace-both",
		"type":           "transfer",
		"debtorId":       "d1",
		"creditorId":     "c1",
		"amount":         15000.0,
		"currency":       "USD",
		"metadata":       metadata,
		"additionalData": additionalData,
	}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal both-keys payload: %v", err)
	}
	return b
}

// TestMessageMetadataHelper covers the small helper directly so the precedence
// rule is locked even without spinning up the worker.
func TestMessageMetadataHelper(t *testing.T) {
	md := map[string]any{"old_balance": 1.0}
	ad := map[string]any{"old_balance": 2.0}

	if got := messageMetadata(md, ad); !reflect.DeepEqual(got, md) {
		t.Errorf("metadata present: expected metadata map, got %v", got)
	}
	if got := messageMetadata(nil, ad); !reflect.DeepEqual(got, ad) {
		t.Errorf("metadata nil: expected additionalData fallback, got %v", got)
	}
	if got := messageMetadata(nil, nil); got != nil {
		t.Errorf("both nil: expected nil, got %v", got)
	}
	// Empty (non-nil) metadata still wins, matching the documented contract:
	// the caller sent the metadata key, so the alternate alias is ignored.
	empty := map[string]any{}
	if got := messageMetadata(empty, ad); !reflect.DeepEqual(got, empty) {
		t.Errorf("empty metadata: expected the empty metadata map to win, got %v", got)
	}
}

// TestProcessTransaction_MetadataKeyHonored proves the documented metadata key
// reaches the rule engine on the async path. Before the fix, encoding/json
// dropped the caller's metadata key, so the drain rule stayed at 0.0 and the
// decision was NALT. Now the drain fires and the decision is ALRT.
func TestProcessTransaction_MetadataKeyHonored(t *testing.T) {
	w, repo, _ := newCapturingWorker(t, alertProcessor(), drainRule())

	payload := drainPayload(t, "metadata", map[string]any{
		"old_balance": 15000.0,
		"new_balance": 0.0,
	})

	if err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-metadata", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false); err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 {
		t.Fatalf("expected 1 SaveEvaluation, got %d", len(saved))
	}
	if saved[0].Eval.Status != domain.StatusAlert {
		t.Fatalf("expected ALRT for the drained account, got %s (score=%.2f)", saved[0].Eval.Status, saved[0].Eval.Score)
	}
	if saved[0].Eval.Score != 1.0 {
		t.Errorf("expected score 1.0 from the real drain signal, got %.2f", saved[0].Eval.Score)
	}
}

// TestProcessTransaction_AdditionalDataStillWorks guards the back-compat alias.
// Producers that already use additionalData must keep working after the fix.
func TestProcessTransaction_AdditionalDataStillWorks(t *testing.T) {
	w, repo, _ := newCapturingWorker(t, alertProcessor(), drainRule())

	payload := drainPayload(t, "additionalData", map[string]any{
		"old_balance": 15000.0,
		"new_balance": 0.0,
	})

	if err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-add", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false); err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 {
		t.Fatalf("expected 1 SaveEvaluation, got %d", len(saved))
	}
	if saved[0].Eval.Status != domain.StatusAlert {
		t.Fatalf("expected ALRT from additionalData alias, got %s", saved[0].Eval.Status)
	}
	if saved[0].Eval.Score != 1.0 {
		t.Errorf("expected score 1.0 from additionalData alias, got %.2f", saved[0].Eval.Score)
	}
}

// TestProcessTransaction_MetadataWinsOverAdditionalData proves the documented
// metadata key wins when a producer sends both. additionalData is intentionally
// bogus so a fallback to it would silently produce the wrong result.
func TestProcessTransaction_MetadataWinsOverAdditionalData(t *testing.T) {
	w, repo, _ := newCapturingWorker(t, alertProcessor(), drainRule())

	payload := payloadWithBoth(t,
		map[string]any{"old_balance": 15000.0, "new_balance": 0.0}, // metadata: real drain
		map[string]any{"old_balance": 0.0, "new_balance": 0.0},     // additionalData: no drain
	)

	if err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-both", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false); err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 {
		t.Fatalf("expected 1 SaveEvaluation, got %d", len(saved))
	}
	if saved[0].Eval.Status != domain.StatusAlert {
		t.Fatalf("expected ALRT from the metadata key (drain), got %s (score=%.2f) — additionalData wrongly won",
			saved[0].Eval.Status, saved[0].Eval.Score)
	}
}

// TestProcessTransaction_UnguardedMetaReadWorks proves the meta map is populated
// from the documented metadata key. Before the fix, an unguarded meta.country
// read errored with "no such key" and the evaluation flipped to a fail-closed
// ALRT with score 0.0. Now the rule evaluates cleanly.
func TestProcessTransaction_UnguardedMetaReadWorks(t *testing.T) {
	w, repo, _ := newCapturingWorker(t, alertProcessor(), metaReadRule())

	payload := drainPayload(t, "metadata", map[string]any{"country": "US"})

	if err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-meta", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false); err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 {
		t.Fatalf("expected 1 SaveEvaluation, got %d", len(saved))
	}
	if saved[0].Eval.Status != domain.StatusAlert {
		t.Fatalf("expected ALRT from the matched country rule, got %s", saved[0].Eval.Status)
	}
	if saved[0].Eval.Score != 1.0 {
		t.Errorf("expected score 1.0 from the country match, got %.2f", saved[0].Eval.Score)
	}
	// The rule must NOT have errored. Before the fix, the result was .err with
	// reason "no such key: country", which is the fail-closed signal we removed.
	for _, r := range saved[0].Eval.RuleResults {
		if r.RuleID == "meta-country-001" && r.SubRuleRef == domain.RuleOutcomeError {
			t.Fatalf("meta.country read errored after the fix: %s — meta map was not populated", r.Reason)
		}
	}
}

// TestProcessTransaction_InvalidBalanceMetadataRejected proves the worker now
// validates metadata the same way POST /evaluate does. A non-numeric balance
// is rejected at admission: processTransaction returns an error, no evaluation
// is saved, and no decision or alert is published. Before the fix, the bad
// value reached the engine and processTransaction failed deep inside EvaluateAll
// with no admission gate, so the same payload behaved differently per path.
func TestProcessTransaction_InvalidBalanceMetadataRejected(t *testing.T) {
	w, repo, busCap := newCapturingWorker(t, alertProcessor(), drainRule())

	payload := drainPayload(t, "metadata", map[string]any{
		"old_balance": 15000.0,
		"new_balance": "0", // a string, not a number or null
	})

	err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-bad", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false)
	if err == nil {
		t.Fatal("expected an error for non-numeric new_balance, got nil")
	}
	if !errors.Is(err, rules.ErrInvalidMetadata) {
		t.Errorf("expected rules.ErrInvalidMetadata, got %v", err)
	}
	if !strings.Contains(err.Error(), "new_balance") {
		t.Errorf("error should name the bad field, got: %v", err)
	}

	if got := len(repo.savedEvals()); got != 0 {
		t.Errorf("expected no SaveEvaluation on admission, got %d", got)
	}
	if got := len(busCap.published()); got != 0 {
		t.Errorf("expected no Publish on admission, got %d", got)
	}
}

// TestProcessTransaction_AlertTopicReceivesDrain covers the end of the async
// pipeline: when the documented metadata key produces a real drain signal, the
// worker must publish the decision and the alert, matching the sync path.
func TestProcessTransaction_AlertTopicReceivesDrain(t *testing.T) {
	w, _, busCap := newCapturingWorker(t, alertProcessor(), drainRule())

	payload := drainPayload(t, "metadata", map[string]any{
		"old_balance": 15000.0,
		"new_balance": 0.0,
	})

	if err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-alert", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false); err != nil {
		t.Fatalf("processTransaction: %v", err)
	}

	pub := busCap.published()
	if got := countPublishes(pub, "tenantA", domain.TopicDecision); got != 1 {
		t.Errorf("decision publishes on tenantA = %d, want 1", got)
	}
	if got := countPublishes(pub, "tenantA", domain.TopicAlert); got != 1 {
		t.Errorf("alert publishes on tenantA = %d, want 1 (drain must alert)", got)
	}
}

// TestProcessTransaction_MetadataParityWithSyncPath proves the two ingestion
// paths produce the same decision and score for the same transaction. The API
// path maps tx.Metadata into AdditionalData; the worker now does the same from
// the documented metadata key. This mirrors TestWorkerEventClock_RealVelocityAgreesWithDirectEvaluate.
func TestProcessTransaction_MetadataParityWithSyncPath(t *testing.T) {
	engine, _ := rules.NewEngine(nil, 5)
	if err := engine.LoadRules(drainRule()); err != nil {
		t.Fatalf("load drain rules: %v", err)
	}

	metadata := map[string]any{"old_balance": 15000.0, "new_balance": 0.0}

	// Sync-path simulation: the API handler maps tx.Metadata -> AdditionalData.
	syncResults, err := engine.EvaluateAll(context.Background(), &rules.EvaluateInput{
		TenantID:       "tenantA",
		TxID:           "tx-sync",
		Type:           "TRANSFER",
		DebtorID:       "d1",
		CreditorID:     "c1",
		Amount:         15000.0,
		Currency:       "USD",
		VelocityWindow: 3600,
		AdditionalData: metadata,
	})
	if err != nil {
		t.Fatalf("sync EvaluateAll: %v", err)
	}
	if syncResults[0].Score != 1.0 {
		t.Fatalf("sync path: expected score 1.0 from the drain, got %.2f", syncResults[0].Score)
	}

	// Async path: the worker reads the same facts from the metadata key.
	w, repo, _ := newCapturingWorker(t, alertProcessor(), nil)
	w.engine = engine

	payload := drainPayload(t, "metadata", metadata)
	if err := w.processTransaction(context.Background(), "tenantA", &domain.Message{
		ID: "msg-parity", TenantID: "tenantA", Topic: domain.TopicTransactionIngested, Payload: payload,
	}, false); err != nil {
		t.Fatalf("worker processTransaction: %v", err)
	}

	saved := repo.savedEvals()
	if len(saved) != 1 {
		t.Fatalf("expected 1 SaveEvaluation, got %d", len(saved))
	}
	workerScore := saved[0].Eval.Score
	if workerScore != syncResults[0].Score {
		t.Errorf("sync and async disagree: sync=%.2f async=%.2f", syncResults[0].Score, workerScore)
	}
	if saved[0].Eval.Status != domain.StatusAlert {
		t.Errorf("expected ALRT on the async path, got %s", saved[0].Eval.Status)
	}
}
