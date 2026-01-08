// Osprey - Transaction monitoring that deploys in 60 seconds.
// Copyright (c) 2025 opensource.finance
// Licensed under the Apache License 2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opensource-finance/osprey/internal/api"
	"github.com/opensource-finance/osprey/internal/bus"
	"github.com/opensource-finance/osprey/internal/cache"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
	"github.com/opensource-finance/osprey/internal/velocity"
	"github.com/opensource-finance/osprey/internal/worker"
)

// Version information (set via ldflags)
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	// Initialize structured logger
	logLevel := slog.LevelInfo
	if os.Getenv("OSPREY_DEBUG") == "true" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Log startup
	slog.Info("starting osprey",
		"version", Version,
		"commit", Commit,
		"build_date", BuildDate,
	)

	// Load configuration
	cfg := domain.DefaultConfig()

	// Check for Pro tier via environment
	if os.Getenv("OSPREY_TIER") == "pro" {
		cfg = domain.ProConfig()
		slog.Info("running in Pro tier mode")
	}

	slog.Info("configuration loaded",
		"tier", cfg.Tier,
		"repository", cfg.Repository.Driver,
		"cache", cfg.Cache.Type,
		"eventbus", cfg.EventBus.Type,
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Initialize Repository
	repo, err := repository.New(cfg.Repository)
	if err != nil {
		slog.Error("failed to initialize repository", "error", err)
		os.Exit(1)
	}
	defer repo.Close()
	slog.Info("repository initialized", "driver", cfg.Repository.Driver)

	// Initialize Cache
	cacheImpl, err := cache.New(cfg.Cache)
	if err != nil {
		slog.Error("failed to initialize cache", "error", err)
		os.Exit(1)
	}
	defer cacheImpl.Close()
	slog.Info("cache initialized", "type", cfg.Cache.Type)

	// Initialize EventBus
	busImpl, err := bus.New(cfg.EventBus)
	if err != nil {
		slog.Error("failed to initialize event bus", "error", err)
		os.Exit(1)
	}
	defer busImpl.Close()
	slog.Info("event bus initialized", "type", cfg.EventBus.Type)

	// Initialize Velocity Service
	velocitySvc := velocity.NewService(repo, cacheImpl)
	slog.Info("velocity service initialized")

	// Initialize Rule Engine with velocity getter
	engine, err := rules.NewEngine(velocitySvc.GetVelocityGetter(), 100)
	if err != nil {
		slog.Error("failed to initialize rule engine", "error", err)
		os.Exit(1)
	}

	// Load rules from database (no hardcoded defaults - configure via API)
	if err := loadRulesFromDatabase(ctx, repo, engine); err != nil {
		slog.Error("failed to load rules", "error", err)
		os.Exit(1)
	}
	slog.Info("rule engine initialized", "rules_count", engine.RulesCount())

	// Initialize Typology Engine
	typologyEngine := rules.NewTypologyEngine()

	// Load typologies from database (no hardcoded defaults - configure via API)
	if err := loadTypologiesFromDatabase(ctx, repo, typologyEngine); err != nil {
		slog.Error("failed to load typologies", "error", err)
		os.Exit(1)
	}
	slog.Info("typology engine initialized", "typologies_count", typologyEngine.TypologyCount())

	// Initialize Decision Processor (TADP)
	processor := tadp.NewProcessor()
	processor.AlertThreshold = 0.7 // Default threshold
	slog.Info("TADP processor initialized", "threshold", processor.AlertThreshold)

	// Initialize async Worker (Pro tier)
	var asyncWorker *worker.Worker
	if cfg.Tier == domain.TierPro || os.Getenv("OSPREY_ASYNC_WORKER") == "true" {
		asyncWorker = worker.NewWorker(busImpl, repo, engine, typologyEngine, processor)

		// Get tenant IDs to process (from environment or default)
		tenantIDs := []string{}
		if envTenants := os.Getenv("OSPREY_TENANTS"); envTenants != "" {
			// Could parse comma-separated list here
			tenantIDs = []string{envTenants}
		}

		workerCfg := worker.Config{
			TenantIDs:   tenantIDs,
			WorkerCount: 5,
		}

		if err := asyncWorker.Start(workerCfg); err != nil {
			slog.Error("failed to start async worker", "error", err)
		} else {
			slog.Info("async worker started", "tenant_count", len(tenantIDs))
		}
	}

	// Initialize Server
	srv := api.NewServer(cfg.Server, repo, cacheImpl, busImpl, engine, typologyEngine, processor, Version)

	// Start Server in goroutine
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("osprey is ready",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
	)

	printBanner(cfg, Version)

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("shutting down...")

	// Stop async worker first
	if asyncWorker != nil {
		if err := asyncWorker.Stop(); err != nil {
			slog.Error("failed to stop async worker", "error", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("osprey shutdown complete")
}

// GlobalTenantID is used for rules that apply to all tenants.
const GlobalTenantID = "*"

// loadRulesFromDatabase loads rules from the database into the engine.
// All rules must be configured via POST /rules API - no hardcoded defaults.
func loadRulesFromDatabase(ctx context.Context, repo domain.Repository, engine *rules.Engine) error {
	dbRules, err := repo.ListRuleConfigs(ctx, GlobalTenantID)
	if err != nil {
		slog.Warn("failed to list rules from database", "error", err)
		return nil // Start with empty rules - they can be added via API
	}

	if len(dbRules) > 0 {
		slog.Info("loading rules from database", "count", len(dbRules))
		return engine.LoadRules(dbRules)
	}

	slog.Info("no rules in database - configure via POST /rules API")
	return nil
}

// loadTypologiesFromDatabase loads typologies from the database into the engine.
// All typologies must be configured via POST /typologies API - no hardcoded defaults.
func loadTypologiesFromDatabase(ctx context.Context, repo domain.Repository, engine *rules.TypologyEngine) error {
	dbTypologies, err := repo.ListTypologies(ctx, GlobalTenantID)
	if err != nil {
		slog.Warn("failed to list typologies from database", "error", err)
		return nil // Start with empty typologies - they can be added via API
	}

	if len(dbTypologies) > 0 {
		slog.Info("loading typologies from database", "count", len(dbTypologies))
		engine.LoadTypologies(dbTypologies)
		return nil
	}

	slog.Info("no typologies in database - configure via POST /typologies API")
	return nil
}

func printBanner(cfg *domain.Config, version string) {
	fmt.Println()
	fmt.Println("  ╔═══════════════════════════════════════════╗")
	fmt.Println("  ║               🦅 OSPREY                   ║")
	fmt.Println("  ║     Transaction Monitoring Engine         ║")
	fmt.Println("  ║      Eyes on every transaction.           ║")
	fmt.Println("  ╚═══════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Version:  %s\n", version)
	fmt.Printf("  Tier:     %s\n", cfg.Tier)
	fmt.Printf("  Server:   http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Println()
	fmt.Println("  Endpoints:")
	fmt.Println("    POST /evaluate          - Evaluate a transaction")
	fmt.Println("    GET  /evaluations/{id}  - Get evaluation by ID")
	fmt.Println("    GET  /transactions/{id} - Get transaction by ID")
	fmt.Println("    GET  /rules             - List all rules")
	fmt.Println("    POST /rules             - Create a new rule")
	fmt.Println("    POST /rules/reload      - Hot-reload rules from database")
	fmt.Println("    GET  /typologies        - List all typologies")
	fmt.Println("    POST /typologies        - Create a new typology")
	fmt.Println("    PUT  /typologies/{id}   - Update a typology")
	fmt.Println("    DELETE /typologies/{id} - Delete a typology")
	fmt.Println("    POST /typologies/reload - Hot-reload typologies")
	fmt.Println("    GET  /health            - Health check")
	fmt.Println()
}
