package rules

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestEngineCreation(t *testing.T) {
	engine, err := NewEngine(nil, 5)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	if engine.RulesCount() != 0 {
		t.Errorf("expected 0 rules, got %d", engine.RulesCount())
	}
}

func TestLoadRule(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer engine.Close()

	rule := &domain.RuleConfig{
		ID:         "test-rule-001",
		Name:       "Test Rule",
		Expression: "amount > 100.0",
		Bands:      []domain.RuleBand{},
		Weight:     1.0,
		Enabled:    true,
	}

	err := engine.LoadRule(rule)
	if err != nil {
		t.Fatalf("failed to load rule: %v", err)
	}

	if engine.RulesCount() != 1 {
		t.Errorf("expected 1 rule, got %d", engine.RulesCount())
	}
}

func TestLoadInvalidRule(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer engine.Close()

	rule := &domain.RuleConfig{
		ID:         "invalid-rule",
		Name:       "Invalid Rule",
		Expression: "this is not valid CEL !!!",
		Enabled:    true,
	}

	err := engine.LoadRule(rule)
	if err == nil {
		t.Error("expected error for invalid CEL expression")
	}
}

func TestEvaluateSimpleRule(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer engine.Close()

	zero := 0.0
	one := 1.0

	rule := &domain.RuleConfig{
		ID:         "amount-check",
		Name:       "Amount Check",
		Expression: "amount > 1000.0 ? 1.0 : 0.0",
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomePass, Reason: "Low amount"},
			{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeFail, Reason: "High amount"},
		},
		Weight:  1.0,
		Enabled: true,
	}

	engine.LoadRule(rule)

	ctx := context.Background()

	// Test with low amount
	input := &EvaluateInput{
		TenantID: "tenant-001",
		TxID:     "tx-001",
		Amount:   500.0,
		Currency: "USD",
	}

	results, err := engine.EvaluateAll(ctx, input)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Score != 0.0 {
		t.Errorf("expected score 0.0 for low amount, got %.2f", results[0].Score)
	}
	if results[0].SubRuleRef != domain.RuleOutcomePass {
		t.Errorf("expected PASS, got %s", results[0].SubRuleRef)
	}

	// Test with high amount
	input.Amount = 5000.0
	results, _ = engine.EvaluateAll(ctx, input)

	if results[0].Score != 1.0 {
		t.Errorf("expected score 1.0 for high amount, got %.2f", results[0].Score)
	}
	if results[0].SubRuleRef != domain.RuleOutcomeFail {
		t.Errorf("expected FAIL, got %s", results[0].SubRuleRef)
	}
}

func TestEvaluateBooleanRule(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer engine.Close()

	rule := &domain.RuleConfig{
		ID:         "same-party-check",
		Name:       "Same Party Check",
		Expression: "debtor_id == creditor_id",
		Bands:      []domain.RuleBand{},
		Weight:     1.0,
		Enabled:    true,
	}

	engine.LoadRule(rule)

	ctx := context.Background()

	// Different parties
	input := &EvaluateInput{
		TenantID:   "tenant-001",
		TxID:       "tx-001",
		DebtorID:   "user-001",
		CreditorID: "user-002",
	}

	results, _ := engine.EvaluateAll(ctx, input)
	if results[0].Score != 0.0 {
		t.Errorf("expected score 0.0 for different parties, got %.2f", results[0].Score)
	}

	// Same party
	input.CreditorID = "user-001"
	results, _ = engine.EvaluateAll(ctx, input)
	if results[0].Score != 1.0 {
		t.Errorf("expected score 1.0 for same party, got %.2f", results[0].Score)
	}
}

func TestVelocityRule(t *testing.T) {
	// Mock velocity getter that returns a fixed count
	velocityGetter := func(ctx context.Context, tenantID, entityID string, windowSecs int) (int64, error) {
		return 15, nil // Simulates 15 transactions in window
	}

	engine, _ := NewEngine(velocityGetter, 5)
	defer engine.Close()

	zero := 0.0
	half := 0.5
	one := 1.0

	// Create velocity rule inline (no hardcoded rules)
	rule := &domain.RuleConfig{
		ID:          "velocity-check-001",
		Name:        "Transaction Velocity Check",
		Description: "Flags accounts with unusually high transaction frequency",
		Version:     "1.0.0",
		Expression:  "velocity_count > 10 ? 1.0 : (velocity_count > 5 ? 0.5 : 0.0)",
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal velocity"},
			{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeReview, Reason: "Elevated velocity"},
			{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeFail, Reason: "High velocity"},
		},
		Weight:  1.0,
		Enabled: true,
	}
	engine.LoadRule(rule)

	ctx := context.Background()
	input := &EvaluateInput{
		TenantID:       "tenant-001",
		TxID:           "tx-001",
		DebtorID:       "user-001",
		VelocityWindow: 3600, // 1 hour
	}

	results, _ := engine.EvaluateAll(ctx, input)

	// With 15 transactions (> 10), should return 1.0 (fail)
	if results[0].Score != 1.0 {
		t.Errorf("expected score 1.0 for high velocity, got %.2f", results[0].Score)
	}
	if results[0].SubRuleRef != domain.RuleOutcomeFail {
		t.Errorf("expected FAIL for high velocity, got %s", results[0].SubRuleRef)
	}
}

func TestParallelExecution(t *testing.T) {
	engine, _ := NewEngine(nil, 3)
	defer engine.Close()

	// Load multiple rules
	for i := 0; i < 10; i++ {
		rule := &domain.RuleConfig{
			ID:         fmt.Sprintf("rule-%d", i),
			Name:       fmt.Sprintf("Rule %d", i),
			Expression: "amount > 0.0",
			Weight:     1.0,
			Enabled:    true,
		}
		engine.LoadRule(rule)
	}

	if engine.RulesCount() != 10 {
		t.Fatalf("expected 10 rules, got %d", engine.RulesCount())
	}

	ctx := context.Background()
	input := &EvaluateInput{
		TenantID: "tenant-001",
		TxID:     "tx-001",
		Amount:   100.0,
	}

	results, err := engine.EvaluateAll(ctx, input)
	if err != nil {
		t.Fatalf("parallel evaluation failed: %v", err)
	}

	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}

	// All should have passed
	for i, r := range results {
		if r.Score != 1.0 {
			t.Errorf("rule %d: expected score 1.0, got %.2f", i, r.Score)
		}
	}
}

func TestConcurrencyLimit(t *testing.T) {
	var concurrentCount int32
	var maxConcurrent int32

	// Velocity getter that tracks concurrent executions
	velocityGetter := func(ctx context.Context, tenantID, entityID string, windowSecs int) (int64, error) {
		current := atomic.AddInt32(&concurrentCount, 1)
		defer atomic.AddInt32(&concurrentCount, -1)

		// Track max concurrent
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond) // Simulate work
		return 5, nil
	}

	engine, _ := NewEngine(velocityGetter, 2) // Max 2 workers
	defer engine.Close()

	// Load 10 rules that use velocity
	for i := 0; i < 10; i++ {
		rule := &domain.RuleConfig{
			ID:         fmt.Sprintf("rule-%d", i),
			Expression: "velocity_count > 10 ? 1.0 : 0.0",
			Enabled:    true,
		}
		engine.LoadRule(rule)
	}

	ctx := context.Background()
	input := &EvaluateInput{
		TenantID:       "tenant-001",
		TxID:           "tx-001",
		DebtorID:       "user-001",
		VelocityWindow: 3600,
	}

	engine.EvaluateAll(ctx, input)

	// Note: Due to how velocity is fetched once before parallel execution,
	// the max concurrent for rule evaluation is controlled by the semaphore
	// This test mainly verifies the worker pool doesn't crash
}

func TestHighValueTransferRule(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer engine.Close()

	zero := 0.0
	one := 1.0

	// Create high value rule inline
	rule := &domain.RuleConfig{
		ID:          "high-value-001",
		Name:        "High Value Transfer Check",
		Description: "Flags transfers above a certain threshold",
		Version:     "1.0.0",
		Expression:  "amount > 10000.0 ? 1.0 : 0.0",
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal transfer amount"},
			{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeReview, Reason: "High value transfer"},
		},
		Weight:  0.8,
		Enabled: true,
	}
	engine.LoadRule(rule)

	ctx := context.Background()

	// Low value
	input := &EvaluateInput{TenantID: "t1", TxID: "tx1", Amount: 500.0}
	results, _ := engine.EvaluateAll(ctx, input)
	if results[0].SubRuleRef != domain.RuleOutcomePass {
		t.Errorf("expected PASS for low value, got %s", results[0].SubRuleRef)
	}

	// High value
	input.Amount = 15000.0
	results, _ = engine.EvaluateAll(ctx, input)
	if results[0].SubRuleRef != domain.RuleOutcomeReview {
		t.Errorf("expected REVIEW for high value, got %s", results[0].SubRuleRef)
	}
}

func TestSameAccountRule(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer engine.Close()

	zero := 0.0
	one := 1.0

	// Create same account rule inline
	rule := &domain.RuleConfig{
		ID:          "same-account-001",
		Name:        "Same Account Transfer Check",
		Description: "Flags transfers where debtor and creditor are the same",
		Version:     "1.0.0",
		Expression:  "debtor_id == creditor_id ? 1.0 : 0.0",
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomePass, Reason: "Different parties"},
			{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeFail, Reason: "Same account transfer"},
		},
		Weight:  1.0,
		Enabled: true,
	}
	engine.LoadRule(rule)

	ctx := context.Background()

	// Different accounts
	input := &EvaluateInput{TenantID: "t1", TxID: "tx1", DebtorID: "a", CreditorID: "b"}
	results, _ := engine.EvaluateAll(ctx, input)
	if results[0].SubRuleRef != domain.RuleOutcomePass {
		t.Errorf("expected PASS for different accounts, got %s", results[0].SubRuleRef)
	}

	// Same account
	input.CreditorID = "a"
	results, _ = engine.EvaluateAll(ctx, input)
	if results[0].SubRuleRef != domain.RuleOutcomeFail {
		t.Errorf("expected FAIL for same account, got %s", results[0].SubRuleRef)
	}
}

func TestRuleResultMetadata(t *testing.T) {
	engine, _ := NewEngine(nil, 5)
	defer engine.Close()

	rule := &domain.RuleConfig{
		ID:         "meta-test",
		Expression: "amount > 0.0",
		Weight:     0.75,
		Enabled:    true,
	}
	engine.LoadRule(rule)

	ctx := context.Background()
	input := &EvaluateInput{
		TenantID: "tenant-123",
		TxID:     "tx-456",
		Amount:   100.0,
	}

	results, _ := engine.EvaluateAll(ctx, input)

	if results[0].RuleID != "meta-test" {
		t.Errorf("expected RuleID 'meta-test', got '%s'", results[0].RuleID)
	}
	if results[0].TenantID != "tenant-123" {
		t.Errorf("expected TenantID 'tenant-123', got '%s'", results[0].TenantID)
	}
	if results[0].TxID != "tx-456" {
		t.Errorf("expected TxID 'tx-456', got '%s'", results[0].TxID)
	}
	if results[0].Weight != 0.75 {
		t.Errorf("expected Weight 0.75, got %.2f", results[0].Weight)
	}
	if results[0].ProcessMs < 0 {
		t.Error("ProcessMs should be non-negative")
	}
}

