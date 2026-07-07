package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
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
