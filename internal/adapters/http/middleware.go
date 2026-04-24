package httpadapter

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"outless/internal/app/auth"
)

// JWTMiddleware validates JWT tokens and injects claims into context.
type JWTMiddleware struct {
	jwtService *auth.JWTService
	logger     *slog.Logger
}

// NewJWTMiddleware constructs a JWT middleware.
func NewJWTMiddleware(jwtService *auth.JWTService, logger *slog.Logger) *JWTMiddleware {
	return &JWTMiddleware{
		jwtService: jwtService,
		logger:     logger,
	}
}

// contextKey is the type for context keys.
type contextKey string

const (
	claimsKey contextKey = "claims"
)

// isPublicPath reports whether the request path is allowed without JWT auth.
func isPublicPath(path string) bool {
	if strings.HasPrefix(path, "/v1/auth/") {
		return true
	}
	if strings.HasPrefix(path, "/v1/sub/") {
		return true
	}
	// OpenAPI schema and docs that huma exposes by default.
	if path == "/openapi.json" || path == "/openapi.yaml" || path == "/docs" || strings.HasPrefix(path, "/docs/") || path == "/schemas" {
		return true
	}
	return false
}

// Wrap returns an http.Handler that validates JWT tokens on non-public paths.
func (m *JWTMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		token := ""
		if authHeader == "" {
			// Browser EventSource / WebSocket cannot attach Authorization; allow token in query.
			if strings.HasSuffix(r.URL.Path, "/sync/stream") || r.URL.Path == "/v1/ws" {
				token = strings.TrimSpace(r.URL.Query().Get("access_token"))
			}
			if token == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}
		}

		if token == "" {
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"message":"` + message + `"}`))
}

// LoggingMiddleware logs all HTTP requests.
type LoggingMiddleware struct {
	logger *slog.Logger
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Hijack delegates to the underlying ResponseWriter so WebSocket upgrades work
// through this middleware (coder/websocket requires http.Hijacker).
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

func extractRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	return host
}

// NewLoggingMiddleware constructs a logging middleware.
func NewLoggingMiddleware(logger *slog.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{logger: logger}
}

// Wrap returns an http.Handler that logs requests.
func (m *LoggingMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)

		message := extractRemoteIP(r.RemoteAddr) + " " + r.URL.Path
		switch {
		case recorder.statusCode >= http.StatusInternalServerError:
			m.logger.Error(message)
		case recorder.statusCode >= http.StatusBadRequest:
			m.logger.Warn(message)
		default:
			m.logger.Info(message)
		}
	})
}

// GetClaims extracts JWT claims from the request context.
func GetClaims(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(claimsKey).(*auth.Claims)
	return claims
}

// RateLimitMiddleware implements simple IP-based rate limiting.
type RateLimitMiddleware struct {
	logger  *slog.Logger
	limiter *rateLimiter
}

// rateLimiter tracks request counts per IP using sliding window.
type rateLimiter struct {
	mu  sync.RWMutex
	ips map[string]*ipState
}

type ipState struct {
	requests []time.Time
}

const (
	maxRequestsPerMinute = 60
	cleanupInterval      = 5 * time.Minute
)

// NewRateLimitMiddleware constructs a rate limiting middleware.
func NewRateLimitMiddleware(logger *slog.Logger) *RateLimitMiddleware {
	rl := &RateLimitMiddleware{
		logger:  logger,
		limiter: &rateLimiter{ips: make(map[string]*ipState)},
	}
	go rl.cleanupOldEntries()
	return rl
}

func (m *RateLimitMiddleware) cleanupOldEntries() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.limiter.mu.Lock()
		now := time.Now()
		for ip, state := range m.limiter.ips {
			// Remove entries with no recent requests
			if len(state.requests) == 0 || now.Sub(state.requests[len(state.requests)-1]) > cleanupInterval {
				delete(m.limiter.ips, ip)
			}
		}
		m.limiter.mu.Unlock()
	}
}

// Wrap returns an http.Handler that enforces rate limiting.
func (m *RateLimitMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractRemoteIP(r.RemoteAddr)

		m.limiter.mu.Lock()
		state, exists := m.limiter.ips[ip]
		if !exists {
			state = &ipState{requests: make([]time.Time, 0)}
			m.limiter.ips[ip] = state
		}

		now := time.Now()
		// Remove requests older than 1 minute
		validIdx := 0
		for i, reqTime := range state.requests {
			if now.Sub(reqTime) < time.Minute {
				validIdx = i
				break
			}
		}
		state.requests = state.requests[validIdx:]

		// Check if limit exceeded
		if len(state.requests) >= maxRequestsPerMinute {
			m.limiter.mu.Unlock()
			m.logger.Warn("rate limit exceeded", slog.String("ip", ip), slog.Int("requests", len(state.requests)))
			writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		state.requests = append(state.requests, now)
		m.limiter.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}
