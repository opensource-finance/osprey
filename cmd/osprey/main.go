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
	"strconv"
	"strings"
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

	// Resolve tier selection.
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OSPREY_TIER"))) {
	case "", "community":
		// Community defaults already applied.
	case "pro":
		cfg = domain.ProConfig()
		slog.Info("running in Pro tier mode")
	case "enterprise":
		slog.Warn("OSPREY_TIER=enterprise is not available in the open-source build; falling back to community tier")
	default:
		slog.Warn("unsupported OSPREY_TIER value; falling back to community tier", "value", os.Getenv("OSPREY_TIER"))
	}

	// Check for Compliance mode via environment
	// Default: Detection mode (fast, simple fraud detection)
	// Compliance mode requires typologies for FATF-aligned evaluation
	if os.Getenv("OSPREY_MODE") == "compliance" {
		cfg.EvaluationMode = domain.ModeCompliance
		slog.Info("running in Compliance mode - typologies required")
	}

	// Apply environment variable overrides for production deployment
	applyEnvOverrides(cfg)

	slog.Info("configuration loaded",
		"tier", cfg.Tier,
		"mode", cfg.EvaluationMode,
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
	processor.AlertThreshold = 0.7              // Default threshold
	processor.Mode = string(cfg.EvaluationMode) // Set mode from config
	slog.Info("TADP processor initialized",
		"mode", processor.Mode,
		"threshold", processor.AlertThreshold,
	)

	// Compliance mode validation: require typologies
	if cfg.EvaluationMode == domain.ModeCompliance && typologyEngine.TypologyCount() == 0 {
		slog.Warn("Compliance mode enabled but no typologies configured",
			"hint", "Create typologies via POST /typologies or switch to Detection mode")
	}

	// Initialize async Worker (Pro tier)
	var asyncWorker *worker.Worker
	if cfg.Tier == domain.TierPro || os.Getenv("OSPREY_ASYNC_WORKER") == "true" {
		asyncWorker = worker.NewWorker(busImpl, repo, engine, typologyEngine, processor, cfg.EvaluationMode)

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
	srv := api.NewServer(cfg.Server, repo, cacheImpl, busImpl, engine, typologyEngine, processor, Version, cfg.EvaluationMode)

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
	fmt.Println("  ║     Real-time Fraud Detection Engine      ║")
	fmt.Println("  ╚═══════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Version:  %s\n", version)
	fmt.Printf("  Tier:     %s\n", cfg.Tier)
	fmt.Printf("  Mode:     %s\n", cfg.EvaluationMode)
	fmt.Printf("  Server:   http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Println()

	// Mode-specific messaging
	if cfg.EvaluationMode == domain.ModeDetection {
		fmt.Println("  Mode: DETECTION (default)")
		fmt.Println("    → Fast, weighted rule scoring")
		fmt.Println("    → No typologies required")
		fmt.Println("    → Ideal for fraud detection, startups")
	} else {
		fmt.Println("  Mode: COMPLIANCE")
		fmt.Println("    → FATF-aligned typology evaluation")
		fmt.Println("    → Full audit trails")
		fmt.Println("    → Ideal for banks, regulated fintechs")
	}
	fmt.Println()
	fmt.Println("  Endpoints:")
	fmt.Println("    POST /evaluate          - Evaluate a transaction")
	fmt.Println("    GET  /evaluations/{id}  - Get evaluation by ID")
	fmt.Println("    GET  /transactions/{id} - Get transaction by ID")
	fmt.Println("    GET  /rules             - List all rules")
	fmt.Println("    POST /rules             - Create a new rule")
	fmt.Println("    POST /rules/reload      - Hot-reload rules from database")
	if cfg.EvaluationMode == domain.ModeCompliance {
		fmt.Println("    GET  /typologies        - List all typologies")
		fmt.Println("    POST /typologies        - Create a new typology")
		fmt.Println("    PUT  /typologies/{id}   - Update a typology")
		fmt.Println("    DELETE /typologies/{id} - Delete a typology")
		fmt.Println("    POST /typologies/reload - Hot-reload typologies")
	}
	fmt.Println("    GET  /health            - Health check")
	fmt.Println()
}

// applyEnvOverrides applies environment variable overrides to the config.
// This enables configuration via environment for Docker/Kubernetes deployments.
func applyEnvOverrides(cfg *domain.Config) {
	// Database driver override
	if driver := os.Getenv("OSPREY_DB_DRIVER"); driver != "" {
		cfg.Repository.Driver = driver
	}

	// PostgreSQL settings
	if host := os.Getenv("OSPREY_POSTGRES_HOST"); host != "" {
		cfg.Repository.PostgresHost = host
	}
	if port := os.Getenv("OSPREY_POSTGRES_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Repository.PostgresPort = p
		}
	}
	if user := os.Getenv("OSPREY_POSTGRES_USER"); user != "" {
		cfg.Repository.PostgresUser = user
	}
	if password := os.Getenv("OSPREY_POSTGRES_PASSWORD"); password != "" {
		cfg.Repository.PostgresPassword = password
	}
	if db := os.Getenv("OSPREY_POSTGRES_DB"); db != "" {
		cfg.Repository.PostgresDB = db
	}
	if sslMode := os.Getenv("OSPREY_POSTGRES_SSLMODE"); sslMode != "" {
		cfg.Repository.PostgresSSLMode = sslMode
	}

	// Cache type override
	if cacheType := os.Getenv("OSPREY_CACHE_TYPE"); cacheType != "" {
		cfg.Cache.Type = cacheType
	}

	// Redis settings
	if addr := os.Getenv("OSPREY_REDIS_ADDR"); addr != "" {
		cfg.Cache.RedisAddr = addr
	}
	if password := os.Getenv("OSPREY_REDIS_PASSWORD"); password != "" {
		cfg.Cache.RedisPassword = password
	}
	if db := os.Getenv("OSPREY_REDIS_DB"); db != "" {
		if d, err := strconv.Atoi(db); err == nil {
			cfg.Cache.RedisDB = d
		}
	}

	// Event bus type override
	if busType := os.Getenv("OSPREY_BUS_TYPE"); busType != "" {
		cfg.EventBus.Type = busType
	}

	// NATS settings
	if url := os.Getenv("OSPREY_NATS_URL"); url != "" {
		cfg.EventBus.NATSUrl = url
	}

	// Server settings
	if port := os.Getenv("OSPREY_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = p
		}
	}
	if host := os.Getenv("OSPREY_HOST"); host != "" {
		cfg.Server.Host = host
	}
}
