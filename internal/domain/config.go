package domain

// Config holds the complete Osprey configuration.
type Config struct {
	// Server settings
	Server ServerConfig `json:"server"`

	// Tier determines feature availability
	Tier Tier `json:"tier"`

	// EvaluationMode determines how transactions are evaluated
	// - "detection": Rules → Weighted Score → Alert (fast, simple)
	// - "compliance": Rules → Typologies → FATF patterns (auditable)
	EvaluationMode EvaluationMode `json:"evaluationMode"`

	// Component configurations
	Repository RepositoryConfig `json:"repository"`
	Cache      CacheConfig      `json:"cache"`
	EventBus   EventBusConfig   `json:"eventBus"`

	// Observability
	Logging LoggingConfig `json:"logging"`
	Tracing TracingConfig `json:"tracing"`
}

// EvaluationMode determines the transaction evaluation strategy.
type EvaluationMode string

const (
	// ModeDetection evaluates rules and aggregates scores directly.
	// Fast, developer-friendly, no typologies required.
	// Use for: Fraud detection, startup MVPs, product teams.
	ModeDetection EvaluationMode = "detection"

	// ModeCompliance requires typologies for FATF-aligned evaluation.
	// Full audit trails, explainability, regulatory compliance.
	// Use for: Banks, regulated fintechs, compliance teams.
	ModeCompliance EvaluationMode = "compliance"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ReadTimeout  int    `json:"readTimeout"`  // seconds
	WriteTimeout int    `json:"writeTimeout"` // seconds
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `json:"level"`  // debug, info, warn, error
	Format string `json:"format"` // json, text
}

// TracingConfig holds OpenTelemetry settings.
type TracingConfig struct {
	Enabled      bool   `json:"enabled"`
	ServiceName  string `json:"serviceName"`
	ExporterType string `json:"exporterType"` // stdout, otlp, jaeger
	Endpoint     string `json:"endpoint"`
}

// Tier represents the product tier.
type Tier string

const (
	// TierCommunity is the free tier with SQLite + channels
	TierCommunity Tier = "community"

	// TierPro is the paid tier with PostgreSQL + NATS + Redis
	TierPro Tier = "pro"

	// TierEnterprise includes multi-node, SSO, etc.
	TierEnterprise Tier = "enterprise"
)

// DefaultConfig returns a default configuration for Community tier.
// Uses Detection mode by default - fast, simple fraud detection.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30,
			WriteTimeout: 30,
		},
		Tier:           TierCommunity,
		EvaluationMode: ModeDetection, // Default: fast fraud detection
		Repository: RepositoryConfig{
			Driver:     "sqlite",
			SQLitePath: "./osprey.db",
		},
		Cache: CacheConfig{
			Type:         "memory",
			LocalMaxSize: 10000,
			LocalTTL:     300, // 5 minutes
		},
		EventBus: EventBusConfig{
			Type:              "channel",
			ChannelBufferSize: 1000,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Tracing: TracingConfig{
			Enabled:     false,
			ServiceName: "osprey",
		},
	}
}

// ProConfig returns a configuration for Pro tier.
// Pro tier supports both Detection and Compliance modes.
// Set OSPREY_MODE=compliance to enable Compliance mode with typologies.
func ProConfig() *Config {
	cfg := DefaultConfig()
	cfg.Tier = TierPro
	// Pro defaults to Detection, but Compliance is available
	cfg.EvaluationMode = ModeDetection
	cfg.Repository = RepositoryConfig{
		Driver:       "postgres",
		PostgresHost: "localhost",
		PostgresPort: 5432,
		PostgresDB:   "osprey",
	}
	cfg.Cache = CacheConfig{
		Type:           "redis",
		RedisAddr:      "localhost:6379",
		EnableTwoPhase: true,
		LocalMaxSize:   1000,
	}
	cfg.EventBus = EventBusConfig{
		Type:              "nats",
		NATSUrl:           "nats://localhost:4222",
		NATSMaxReconnects: 10,
		NATSReconnectWait: 5,
	}
	cfg.Tracing.Enabled = true
	return cfg
}
