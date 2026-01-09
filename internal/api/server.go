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
	router  *chi.Mux
	handler *Handler
	server  *http.Server
	config  domain.ServerConfig
}

// NewServer creates a new API server.
func NewServer(cfg domain.ServerConfig, repo domain.Repository, cache domain.Cache, bus domain.EventBus, engine *rules.Engine, typologyEngine *rules.TypologyEngine, processor *tadp.Processor, version string, mode domain.EvaluationMode) *Server {
	handler := NewHandler(repo, cache, bus, engine, typologyEngine, processor, version, mode)
	router := chi.NewRouter()

	// Global middleware stack
	router.Use(CORSMiddleware)         // CORS for browser clients
	router.Use(RecoverMiddleware)      // Recover from panics
	router.Use(TracingMiddleware)      // OpenTelemetry tracing
	router.Use(LoggingMiddleware)      // Request logging
	router.Use(middleware.RealIP)      // Extract real IP
	router.Use(middleware.Compress(5)) // Gzip compression

	// Health endpoints (no tenant required)
	router.Get("/health", handler.Health)
	router.Get("/ready", handler.Ready)

	// API routes (tenant required)
	router.Route("/", func(r chi.Router) {
		r.Use(TenantMiddleware)

		// Transaction evaluation
		r.Post("/evaluate", handler.Evaluate)

		// Evaluation retrieval
		r.Get("/evaluations/{id}", handler.GetEvaluation)

		// Transaction retrieval
		r.Get("/transactions/{id}", handler.GetTransaction)

		// Rule management
		r.Get("/rules", handler.ListRules)
		r.Get("/rules/{id}", handler.GetRule)
		r.Post("/rules", handler.CreateRule)
		r.Post("/rules/reload", handler.ReloadRules)

		// Typology management
		r.Get("/typologies", handler.ListTypologies)
		r.Get("/typologies/{id}", handler.GetTypology)
		r.Post("/typologies", handler.CreateTypology)
		r.Put("/typologies/{id}", handler.UpdateTypology)
		r.Delete("/typologies/{id}", handler.DeleteTypology)
		r.Post("/typologies/reload", handler.ReloadTypologies)
	})

	return &Server{
		router:  router,
		handler: handler,
		config:  cfg,
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

// Handler returns the handler for testing.
func (s *Server) Handler() *Handler {
	return s.handler
}
