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

// handleVersionFlag prints version info and reports whether the process should
// exit. Runs before config/logging so it works without OSPREY_ADMIN_TOKEN or
// any other environment.
func handleVersionFlag() bool {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("osprey %s (commit %s, built %s)\n", Version, Commit, BuildDate)
			return true
		}
	}
	return false
}

// initLogger installs the default structured logger.
func initLogger() {
	logLevel := slog.LevelInfo
	if os.Getenv("OSPREY_DEBUG") == "true" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)
}

// resolveTierConfig returns the base configuration for the selected OSPREY_TIER.
func resolveTierConfig() *domain.Config {
	cfg := domain.DefaultConfig()
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
	return cfg
}

func main() {
	if handleVersionFlag() {
		return
	}

	initLogger()

	// Log startup
	slog.Info("starting osprey",
		"version", Version,
		"commit", Commit,
		"build_date", BuildDate,
	)

	// Load configuration and resolve tier selection.
	cfg := resolveTierConfig()

	if err := applyModeOverride(cfg); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// Apply environment variable overrides for production deployment
	if err := applyEnvOverrides(cfg); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	if strings.TrimSpace(cfg.Server.AdminToken) == "" {
		slog.Error("OSPREY_ADMIN_TOKEN is required for configuration mutation endpoints")
		os.Exit(1)
	}

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
	defer func() { _ = repo.Close() }()
	slog.Info("repository initialized", "driver", cfg.Repository.Driver)

	// Initialize Cache
	cacheImpl, err := cache.New(cfg.Cache)
	if err != nil {
		slog.Error("failed to initialize cache", "error", err)
		os.Exit(1)
	}
	defer func() { _ = cacheImpl.Close() }()
	slog.Info("cache initialized", "type", cfg.Cache.Type)

	// Initialize EventBus
	busImpl, err := bus.New(cfg.EventBus)
	if err != nil {
		slog.Error("failed to initialize event bus", "error", err)
		os.Exit(1)
	}
	defer func() { _ = busImpl.Close() }()
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
	// Wire richer velocity aggregates (count + amount-sum + distinct-counterparties).
	engine.SetAggregatesGetter(func(ctx context.Context, tenantID, entityID string, windowSecs int) (rules.VelocityAggregates, error) {
		a, err := velocitySvc.GetAggregates(ctx, tenantID, entityID, windowSecs)
		if err != nil {
			return rules.VelocityAggregates{}, err
		}
		return rules.VelocityAggregates{Count: a.Count, AmountSum: a.AmountSum, DistinctCreditors: a.DistinctCreditors}, nil
	})

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
			tenantIDs = parseTenantIDs(envTenants)
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

func parseTenantIDs(value string) []string {
	parts := strings.Split(value, ",")
	tenantIDs := make([]string, 0, len(parts))
	for _, part := range parts {
		tenantID := strings.TrimSpace(part)
		if tenantID != "" {
			tenantIDs = append(tenantIDs, tenantID)
		}
	}
	return tenantIDs
}

// loadRulesFromDatabase loads rules from the database into the engine.
// All rules must be configured via POST /rules API - no hardcoded defaults.
func loadRulesFromDatabase(ctx context.Context, repo domain.Repository, engine *rules.Engine) error {
	dbRules, err := repo.ListRuleConfigs(ctx, domain.GlobalTenantID)
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
	dbTypologies, err := repo.ListTypologies(ctx, domain.GlobalTenantID)
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
	fmt.Println("    POST /rules             - Save and activate a rule")
	fmt.Println("    POST /rules/reload      - Reload rules from database")
	if cfg.EvaluationMode == domain.ModeCompliance {
		fmt.Println("    GET  /typologies        - List all typologies")
		fmt.Println("    POST /typologies        - Save and activate a typology")
		fmt.Println("    PUT  /typologies/{id}   - Update and activate a typology")
		fmt.Println("    DELETE /typologies/{id} - Delete and deactivate a typology")
		fmt.Println("    POST /typologies/reload - Reload typologies from database")
	}
	fmt.Println("    GET  /health            - Health check")
	fmt.Println()
}

// applyEnvOverrides applies environment variable overrides to the config.
// This enables configuration via environment for Docker/Kubernetes deployments.
func applyModeOverride(cfg *domain.Config) error {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("OSPREY_MODE")))
	switch mode {
	case "":
		return nil
	case string(domain.ModeDetection):
		cfg.EvaluationMode = domain.ModeDetection
	case string(domain.ModeCompliance):
		cfg.EvaluationMode = domain.ModeCompliance
		slog.Info("running in Compliance mode - typologies required")
	default:
		return fmt.Errorf("OSPREY_MODE must be %q or %q", domain.ModeDetection, domain.ModeCompliance)
	}
	return nil
}

func applyEnvOverrides(cfg *domain.Config) error {
	// Database driver override
	if driver := strings.TrimSpace(os.Getenv("OSPREY_DB_DRIVER")); driver != "" {
		cfg.Repository.Driver = driver
	}
	if sqlitePath := strings.TrimSpace(os.Getenv("OSPREY_SQLITE_PATH")); sqlitePath != "" {
		cfg.Repository.SQLitePath = sqlitePath
	}

	// PostgreSQL settings
	if host := strings.TrimSpace(os.Getenv("OSPREY_POSTGRES_HOST")); host != "" {
		cfg.Repository.PostgresHost = host
	}
	if p, err := parseIntEnv("OSPREY_POSTGRES_PORT"); err != nil {
		return err
	} else if p > 0 {
		cfg.Repository.PostgresPort = p
	}
	if user := strings.TrimSpace(os.Getenv("OSPREY_POSTGRES_USER")); user != "" {
		cfg.Repository.PostgresUser = user
	}
	if password := os.Getenv("OSPREY_POSTGRES_PASSWORD"); password != "" {
		cfg.Repository.PostgresPassword = password
	}
	if db := strings.TrimSpace(os.Getenv("OSPREY_POSTGRES_DB")); db != "" {
		cfg.Repository.PostgresDB = db
	}
	if sslMode := strings.TrimSpace(os.Getenv("OSPREY_POSTGRES_SSLMODE")); sslMode != "" {
		cfg.Repository.PostgresSSLMode = sslMode
	}

	// Cache type override
	if cacheType := strings.TrimSpace(os.Getenv("OSPREY_CACHE_TYPE")); cacheType != "" {
		cfg.Cache.Type = cacheType
	}

	// Redis settings
	if addr := strings.TrimSpace(os.Getenv("OSPREY_REDIS_ADDR")); addr != "" {
		cfg.Cache.RedisAddr = addr
	}
	if password := os.Getenv("OSPREY_REDIS_PASSWORD"); password != "" {
		cfg.Cache.RedisPassword = password
	}
	if d, ok, err := parseNonNegativeIntEnv("OSPREY_REDIS_DB"); err != nil {
		return err
	} else if ok {
		cfg.Cache.RedisDB = d
	}

	// Event bus type override
	if busType := strings.TrimSpace(os.Getenv("OSPREY_BUS_TYPE")); busType != "" {
		cfg.EventBus.Type = busType
	}

	// NATS settings
	if url := strings.TrimSpace(os.Getenv("OSPREY_NATS_URL")); url != "" {
		cfg.EventBus.NATSUrl = url
	}

	// Server settings
	if p, err := parseIntEnv("OSPREY_PORT"); err != nil {
		return err
	} else if p > 0 {
		cfg.Server.Port = p
	}
	if adminToken := strings.TrimSpace(os.Getenv("OSPREY_ADMIN_TOKEN")); adminToken != "" {
		cfg.Server.AdminToken = adminToken
	}
	if host := strings.TrimSpace(os.Getenv("OSPREY_HOST")); host != "" {
		cfg.Server.Host = host
	}
	if v := strings.TrimSpace(os.Getenv("OSPREY_RATE_LIMIT_RPS")); v != "" {
		rps, err := strconv.ParseFloat(v, 64)
		if err != nil || rps < 0 {
			return fmt.Errorf("OSPREY_RATE_LIMIT_RPS must be a non-negative number")
		}
		cfg.Server.RateLimitRPS = rps
	}
	if b, ok, err := parseNonNegativeIntEnv("OSPREY_RATE_LIMIT_BURST"); err != nil {
		return err
	} else if ok {
		cfg.Server.RateLimitBurst = b
	}
	return nil
}

func parseIntEnv(name string) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return value, nil
}

func parseNonNegativeIntEnv(name string) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be an integer", name)
	}
	if value < 0 {
		return 0, false, fmt.Errorf("%s must be zero or greater", name)
	}
	return value, true, nil
}
