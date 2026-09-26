package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
)

func TestParseTenantIDs(t *testing.T) {
	got := parseTenantIDs(" tenant-a,tenant-b ,, tenant-c ")
	want := []string{"tenant-a", "tenant-b", "tenant-c"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestApplyModeOverride(t *testing.T) {
	t.Run("AcceptsCaseInsensitiveCompliance", func(t *testing.T) {
		t.Setenv("OSPREY_MODE", " Compliance ")
		cfg := domain.DefaultConfig()

		if err := applyModeOverride(cfg); err != nil {
			t.Fatalf("expected compliance mode to parse: %v", err)
		}
		if cfg.EvaluationMode != domain.ModeCompliance {
			t.Fatalf("expected compliance mode, got %s", cfg.EvaluationMode)
		}
	})

	t.Run("RejectsInvalidMode", func(t *testing.T) {
		t.Setenv("OSPREY_MODE", "audit")
		cfg := domain.DefaultConfig()

		err := applyModeOverride(cfg)
		if err == nil {
			t.Fatal("expected invalid mode to fail")
		}
		if !strings.Contains(err.Error(), "OSPREY_MODE") {
			t.Fatalf("expected OSPREY_MODE error, got %v", err)
		}
	})
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Run("AppliesTrimmedNumericAndStringOverrides", func(t *testing.T) {
		t.Setenv("OSPREY_PORT", " 9090 ")
		t.Setenv("OSPREY_POSTGRES_PORT", " 5544 ")
		t.Setenv("OSPREY_REDIS_DB", "0")
		t.Setenv("OSPREY_HOST", " 127.0.0.1 ")
		t.Setenv("OSPREY_ADMIN_TOKEN", " sandbox-token ")

		cfg := domain.DefaultConfig()
		if err := applyEnvOverrides(cfg); err != nil {
			t.Fatalf("expected env overrides to apply: %v", err)
		}

		if cfg.Server.Port != 9090 {
			t.Fatalf("expected server port 9090, got %d", cfg.Server.Port)
		}
		if cfg.Repository.PostgresPort != 5544 {
			t.Fatalf("expected postgres port 5544, got %d", cfg.Repository.PostgresPort)
		}
		if cfg.Cache.RedisDB != 0 {
			t.Fatalf("expected redis db 0, got %d", cfg.Cache.RedisDB)
		}
		if cfg.Server.Host != "127.0.0.1" {
			t.Fatalf("expected trimmed host, got %q", cfg.Server.Host)
		}
		if cfg.Server.AdminToken != "sandbox-token" {
			t.Fatalf("expected trimmed admin token, got %q", cfg.Server.AdminToken)
		}
	})

	t.Run("RejectsInvalidIntegerOverrides", func(t *testing.T) {
		t.Setenv("OSPREY_PORT", "not-a-port")
		cfg := domain.DefaultConfig()

		err := applyEnvOverrides(cfg)
		if err == nil {
			t.Fatal("expected invalid port to fail")
		}
		if !strings.Contains(err.Error(), "OSPREY_PORT") {
			t.Fatalf("expected OSPREY_PORT error, got %v", err)
		}
	})

	t.Run("PreservesSecretWhitespace", func(t *testing.T) {
		t.Setenv("OSPREY_POSTGRES_PASSWORD", " secret with spaces ")
		t.Setenv("OSPREY_REDIS_PASSWORD", " redis secret ")

		cfg := domain.DefaultConfig()
		if err := applyEnvOverrides(cfg); err != nil {
			t.Fatalf("expected env overrides to apply: %v", err)
		}

		if cfg.Repository.PostgresPassword != " secret with spaces " {
			t.Fatalf("postgres password whitespace was not preserved")
		}
		if cfg.Cache.RedisPassword != " redis secret " {
			t.Fatalf("redis password whitespace was not preserved")
		}
	})
}

// loaderRepo is a configurable fake domain.Repository for the DB loader tests.
// It embeds domain.Repository so only the methods exercised by the loaders need
// to be implemented, mirroring the fake-repo pattern used in internal/api and
// internal/worker tests.
type loaderRepo struct {
	domain.Repository

	rules      []*domain.RuleConfig
	typologies []*domain.Typology

	listRulesErr      error
	listTypologiesErr error

	listRulesCalls      int
	listTypologiesCalls int
}

func (r *loaderRepo) ListRuleConfigs(_ context.Context, _ string) ([]*domain.RuleConfig, error) {
	r.listRulesCalls++
	if r.listRulesErr != nil {
		return nil, r.listRulesErr
	}
	return r.rules, nil
}

func (r *loaderRepo) ListTypologies(_ context.Context, _ string) ([]*domain.Typology, error) {
	r.listTypologiesCalls++
	if r.listTypologiesErr != nil {
		return nil, r.listTypologiesErr
	}
	return r.typologies, nil
}

func newTestEngine(t *testing.T) *rules.Engine {
	t.Helper()
	engine, err := rules.NewEngine(nil, 5)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return engine
}

func TestLoadRulesFromDatabase_PropagatesListError(t *testing.T) {
	// Regression guard: a ListRuleConfigs failure must fail startup loudly
	// rather than being swallowed and leaving the engine inert with zero
	// rules (which would classify every transaction as no_alert).
	dbErr := errors.New("db connection refused")
	repo := &loaderRepo{listRulesErr: dbErr}
	engine := newTestEngine(t)

	err := loadRulesFromDatabase(context.Background(), repo, engine)
	if err == nil {
		t.Fatal("expected loadRulesFromDatabase to return an error when ListRuleConfigs fails, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected returned error to wrap the underlying DB error; got %v", err)
	}
	if !strings.Contains(err.Error(), "list rules from database") {
		t.Fatalf("expected error message to mention the operation; got %v", err)
	}
	if repo.listRulesCalls != 1 {
		t.Fatalf("expected ListRuleConfigs to be called once, got %d", repo.listRulesCalls)
	}
	if engine.RulesCount() != 0 {
		t.Fatalf("expected zero rules loaded on error, got %d", engine.RulesCount())
	}
}

func TestLoadRulesFromDatabase_EmptyDatabaseStartsCleanly(t *testing.T) {
	// Onboarding guarantee: a legitimately empty table is a benign first-run
	// state and must still start the service so operators can POST /rules.
	repo := &loaderRepo{rules: nil}
	engine := newTestEngine(t)

	if err := loadRulesFromDatabase(context.Background(), repo, engine); err != nil {
		t.Fatalf("expected nil error for legitimately empty rules table, got %v", err)
	}
	if engine.RulesCount() != 0 {
		t.Fatalf("expected zero rules for empty table, got %d", engine.RulesCount())
	}
	if repo.listRulesCalls != 1 {
		t.Fatalf("expected ListRuleConfigs to be called once, got %d", repo.listRulesCalls)
	}
}

func TestLoadTypologiesFromDatabase_PropagatesListError(t *testing.T) {
	// Regression guard: a ListTypologies failure must fail startup loudly
	// rather than being swallowed and leaving the typology engine inert.
	dbErr := errors.New("context deadline exceeded")
	repo := &loaderRepo{listTypologiesErr: dbErr}
	engine := rules.NewTypologyEngine()

	err := loadTypologiesFromDatabase(context.Background(), repo, engine)
	if err == nil {
		t.Fatal("expected loadTypologiesFromDatabase to return an error when ListTypologies fails, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected returned error to wrap the underlying DB error; got %v", err)
	}
	if !strings.Contains(err.Error(), "list typologies from database") {
		t.Fatalf("expected error message to mention the operation; got %v", err)
	}
	if repo.listTypologiesCalls != 1 {
		t.Fatalf("expected ListTypologies to be called once, got %d", repo.listTypologiesCalls)
	}
	if engine.TypologyCount() != 0 {
		t.Fatalf("expected zero typologies loaded on error, got %d", engine.TypologyCount())
	}
}

func TestLoadTypologiesFromDatabase_EmptyDatabaseStartsCleanly(t *testing.T) {
	// Onboarding guarantee: a legitimately empty typology table must still
	// start the service so operators can POST /typologies.
	repo := &loaderRepo{typologies: nil}
	engine := rules.NewTypologyEngine()

	if err := loadTypologiesFromDatabase(context.Background(), repo, engine); err != nil {
		t.Fatalf("expected nil error for legitimately empty typologies table, got %v", err)
	}
	if engine.TypologyCount() != 0 {
		t.Fatalf("expected zero typologies for empty table, got %d", engine.TypologyCount())
	}
	if repo.listTypologiesCalls != 1 {
		t.Fatalf("expected ListTypologies to be called once, got %d", repo.listTypologiesCalls)
	}
}
