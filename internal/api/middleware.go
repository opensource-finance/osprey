package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Context keys for tenant and trace propagation.
type contextKey string

const (
	// TenantIDKey is the context key for tenant ID.
	TenantIDKey contextKey = "tenantID"

	// TraceIDKey is the context key for trace ID.
	TraceIDKey contextKey = "traceID"

	// RequestIDKey is the context key for request ID.
	RequestIDKey contextKey = "requestID"

	// TenantIDHeader is the HTTP header for tenant ID.
	TenantIDHeader = "X-Tenant-ID"

	// RequestIDHeader is the HTTP header for request ID.
	RequestIDHeader = "X-Request-ID"

	// TraceIDHeader is the HTTP header for trace ID.
	TraceIDHeader = "X-Trace-ID"
)

var tracer = otel.Tracer("osprey-api")

// TenantMiddleware extracts tenant ID from the X-Tenant-ID header
// and adds it to the request context.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get(TenantIDHeader)
		if tenantID == "" {
			http.Error(w, `{"error":"X-Tenant-ID header is required"}`, http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TracingMiddleware creates OpenTelemetry spans and propagates trace context.
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate or extract request ID
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Start span
		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path,
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.path", r.URL.Path),
				attribute.String("request.id", requestID),
			),
		)
		defer span.End()

		// Get trace ID from span
		traceID := span.SpanContext().TraceID().String()
		if !span.SpanContext().TraceID().IsValid() {
			traceID = requestID
		}

		// Add to context
		ctx = context.WithValue(ctx, RequestIDKey, requestID)
		ctx = context.WithValue(ctx, TraceIDKey, traceID)

		// Set response headers
		w.Header().Set(RequestIDHeader, requestID)
		w.Header().Set(TraceIDHeader, traceID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggingMiddleware logs HTTP requests with structured logging.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		// Extract context values for logging
		tenantID, _ := r.Context().Value(TenantIDKey).(string)
		requestID, _ := r.Context().Value(RequestIDKey).(string)
		traceID, _ := r.Context().Value(TraceIDKey).(string)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", duration.Milliseconds(),
			"tenant_id", tenantID,
			"request_id", requestID,
			"trace_id", traceID,
		)
	})
}

// CORSMiddleware handles Cross-Origin Resource Sharing for browser clients.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from any origin in development
		// In production, this should be restricted to specific origins
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Tenant-ID, X-Request-ID, X-Trace-ID, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Trace-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RecoverMiddleware recovers from panics and returns 500.
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"path", r.URL.Path,
				)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// GetTenantID extracts tenant ID from context.
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(TenantIDKey).(string); ok {
		return v
	}
	return ""
}

// GetTraceID extracts trace ID from context.
func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}

