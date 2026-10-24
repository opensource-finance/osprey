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

	t.Run("StudioOnByDefaultAndCanBeTurnedOff", func(t *testing.T) {
		cfg := domain.DefaultConfig()
		if err := applyEnvOverrides(cfg); err != nil {
			t.Fatalf("expected env overrides to apply: %v", err)
		}
		if !cfg.Server.Studio {
			t.Fatal("expected the studio to be on by default")
		}

		t.Setenv("OSPREY_STUDIO", " false ")
		if err := applyEnvOverrides(cfg); err != nil {
			t.Fatalf("expected env overrides to apply: %v", err)
		}
		if cfg.Server.Studio {
			t.Fatal("expected OSPREY_STUDIO=false to turn the studio off")
		}

		t.Setenv("OSPREY_STUDIO", "maybe")
		if err := applyEnvOverrides(cfg); err == nil || !strings.Contains(err.Error(), "OSPREY_STUDIO") {
			t.Fatalf("expected OSPREY_STUDIO error, got %v", err)
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

func TestApplyDetectionOverrides(t *testing.T) {
	t.Run("Defaults", func(t *testing.T) {
		cfg := domain.DefaultConfig()
		if err := applyEnvOverrides(cfg); err != nil {
			t.Fatal(err)
		}
		want := domain.DetectionConfig{AlertThreshold: 0.7, Scoring: "sum", VelocityClock: "received"}
		if cfg.Detection != want {
			t.Fatalf("got %+v, want %+v", cfg.Detection, want)
		}
	})

	t.Run("Overrides", func(t *testing.T) {
		t.Setenv("OSPREY_ALERT_THRESHOLD", " 0.5 ")
		t.Setenv("OSPREY_SCORING", "Average")
		t.Setenv("OSPREY_VELOCITY_CLOCK", "event")
		t.Setenv("OSPREY_SANCTIONS_DIR", " data/ofac ")
		cfg := domain.DefaultConfig()
		if err := applyEnvOverrides(cfg); err != nil {
			t.Fatal(err)
		}
		want := domain.DetectionConfig{AlertThreshold: 0.5, Scoring: "average", VelocityClock: "event", SanctionsDir: "data/ofac"}
		if cfg.Detection != want {
			t.Fatalf("got %+v, want %+v", cfg.Detection, want)
		}
	})

	for name, env := range map[string][2]string{
		"ThresholdZero":      {"OSPREY_ALERT_THRESHOLD", "0"},
		"ThresholdAboveOne":  {"OSPREY_ALERT_THRESHOLD", "1.5"},
		"ThresholdNotNumber": {"OSPREY_ALERT_THRESHOLD", "high"},
		"UnknownScoring":     {"OSPREY_SCORING", "max"},
		"UnknownClock":       {"OSPREY_VELOCITY_CLOCK", "wall"},
	} {
		t.Run("Rejects"+name, func(t *testing.T) {
			t.Setenv(env[0], env[1])
			err := applyEnvOverrides(domain.DefaultConfig())
			if err == nil || !strings.Contains(err.Error(), env[0]) {
				t.Fatalf("want %s error, got %v", env[0], err)
			}
		})
	}
}

// loaderRepo is a fake domain.Repository for the DB loader tests.
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
	// A ListRuleConfigs error must fail startup.
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
	// An empty rules table still starts.
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
	// A ListTypologies error must fail startup.
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
	// An empty typologies table still starts.
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

func TestNewProcessorUsesDetectionConfig(t *testing.T) {
	cfg := domain.DefaultConfig()
	cfg.EvaluationMode = domain.ModeCompliance
	cfg.Detection.AlertThreshold = 0.55
	cfg.Detection.Scoring = "average"
	p := newProcessor(cfg)
	if p.AlertThreshold != 0.55 || p.Scoring != "average" || p.Mode != "compliance" {
		t.Fatalf("got threshold %v, scoring %q, mode %q", p.AlertThreshold, p.Scoring, p.Mode)
	}

	cfg.Detection.AlertThreshold = 0
	if p := newProcessor(cfg); p.AlertThreshold != 0.7 {
		t.Fatalf("threshold 0 must keep the default 0.7, got %v", p.AlertThreshold)
	}
}
