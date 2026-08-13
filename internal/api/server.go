package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

// Server represents the HTTP API server.
type Server struct {
	router *chi.Mux
	server *http.Server
	config domain.ServerConfig
}

// NewServer creates a new API server.
func NewServer(cfg domain.ServerConfig, repo domain.Repository, cache domain.Cache, bus domain.EventBus, engine *rules.Engine, typologyEngine *rules.TypologyEngine, processor *tadp.Processor, version string, mode domain.EvaluationMode) *Server {
	handler := NewHandler(repo, cache, bus, engine, typologyEngine, processor, version, mode)
	router := chi.NewRouter()

	// Global middleware stack
	router.Use(CORSMiddleware)    // CORS for browser clients
	router.Use(RecoverMiddleware) // Recover from panics
	router.Use(TracingMiddleware) // OpenTelemetry tracing
	router.Use(LoggingMiddleware) // Request logging
	// No middleware.RealIP: chi deprecated it in v5.3.0 because it rewrites
	// RemoteAddr from client-controlled headers whether or not a proxy sets
	// them (GHSA-3fxj-6jh8-hvhx and two siblings). Nothing here reads
	// RemoteAddr, so it was spoofable input feeding nothing. Anything that
	// needs the client IP behind a proxy must parse a header the deployment
	// actually controls, not trust the leftmost hop.
	router.Use(middleware.Compress(5)) // Gzip compression

	// Health endpoints (no tenant required)
	router.Get("/health", handler.Health)
	router.Get("/ready", handler.Ready)

	// API routes (tenant required)
	router.Route("/", func(r chi.Router) {
		r.Use(TenantMiddleware)
		r.Use(RateLimitMiddleware(cfg.RateLimitRPS, cfg.RateLimitBurst)) // per-tenant; disabled when RPS <= 0

		// Transaction evaluation
		r.Post("/evaluate", handler.Evaluate)

		// Evaluation retrieval
		r.Get("/evaluations/{id}", handler.GetEvaluation)

		// Transaction retrieval
		r.Get("/transactions/{id}", handler.GetTransaction)

		// Rule management
		r.Get("/rules", handler.ListRules)
		r.Get("/rules/variables", handler.ListVariables) // static route before /rules/{id}
		r.Get("/rules/{id}", handler.GetRule)
		r.Group(func(r chi.Router) {
			r.Use(AdminMiddleware(cfg.AdminToken))
			r.Post("/rules", handler.CreateRule)
			r.Put("/rules/{id}", handler.UpdateRule)
			r.Delete("/rules/{id}", handler.DeleteRule)
			r.Post("/rules/reload", handler.ReloadRules)
		})

		// Typology management
		r.Get("/typologies", handler.ListTypologies)
		r.Get("/typologies/{id}", handler.GetTypology)
		r.Group(func(r chi.Router) {
			r.Use(AdminMiddleware(cfg.AdminToken))
			r.Post("/typologies", handler.CreateTypology)
			r.Put("/typologies/{id}", handler.UpdateTypology)
			r.Delete("/typologies/{id}", handler.DeleteTypology)
			r.Post("/typologies/reload", handler.ReloadTypologies)
		})
	})

	return &Server{
		router: router,
		config: cfg,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// Router returns the Chi router for testing.
func (s *Server) Router() *chi.Mux {
	return s.router
}
