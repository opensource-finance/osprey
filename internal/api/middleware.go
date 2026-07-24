package api

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
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

	// AdminTokenHeader is an alternate token header for configuration changes.
	AdminTokenHeader = "X-Osprey-Admin-Token" //nolint:gosec // G101: header name, not a credential
)

var tracer = otel.Tracer("osprey-api")

// TenantMiddleware extracts tenant ID from the X-Tenant-ID header
// and adds it to the request context.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := strings.TrimSpace(r.Header.Get(TenantIDHeader))
		if tenantID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "X-Tenant-ID header is required",
			})
			return
		}

		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminMiddleware protects configuration mutation endpoints.
func AdminMiddleware(adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminToken == "" {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"error": "admin token is not configured",
				})
				return
			}

			if !validAdminToken(r, adminToken) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "admin token is invalid or missing",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validAdminToken(r *http.Request, expected string) bool {
	provided := r.Header.Get(AdminTokenHeader)
	if provided == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			provided = strings.TrimSpace(auth[len("bearer "):])
		}
	}

	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
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
		if tenantID == "" {
			tenantID = r.Header.Get(TenantIDHeader)
		}
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Tenant-ID, X-Request-ID, X-Trace-ID, X-Osprey-Admin-Token, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Trace-ID")
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
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// maxRateLimitTenants bounds the number of per-tenant limiters held in memory
// so an attacker sending many distinct X-Tenant-ID values cannot grow the map
// without limit. When exceeded, the map is reset (buckets refill).
const maxRateLimitTenants = 10000

type rateLimiter struct {
	limit    rate.Limit
	burst    int
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

func (rl *rateLimiter) get(tenantID string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if len(rl.limiters) >= maxRateLimitTenants {
		rl.limiters = make(map[string]*rate.Limiter)
	}
	l, ok := rl.limiters[tenantID]
	if !ok {
		l = rate.NewLimiter(rl.limit, rl.burst)
		rl.limiters[tenantID] = l
	}
	return l
}

// RateLimitMiddleware applies a per-tenant token-bucket limit. A non-positive
// rps disables limiting entirely (the middleware is a pass-through), so the
// default deployment and load tests are unthrottled. Must run after
// TenantMiddleware so the tenant ID is available on the context.
func RateLimitMiddleware(rps float64, burst int) func(http.Handler) http.Handler {
	if rps <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	if burst <= 0 {
		burst = max(int(rps), 1)
	}
	rl := &rateLimiter{
		limit:    rate.Limit(rps),
		burst:    burst,
		limiters: make(map[string]*rate.Limiter),
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := GetTenantID(r.Context())
			if tenantID == "" {
				tenantID = strings.TrimSpace(r.Header.Get(TenantIDHeader))
			}
			if !rl.get(tenantID).Allow() {
				w.Header().Set("Retry-After", "1")
				writeJSON(w, http.StatusTooManyRequests, map[string]string{
					"error": "rate limit exceeded; retry later",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
