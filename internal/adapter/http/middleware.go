package http

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"outless/internal/service"
)

// JWTMiddleware validates JWT tokens and injects claims into context.
type JWTMiddleware struct {
	jwtService *service.JWTService
	logger     *slog.Logger
}

// NewJWTMiddleware constructs a JWT middleware.
func NewJWTMiddleware(jwtService *service.JWTService, logger *slog.Logger) *JWTMiddleware {
	return &JWTMiddleware{
		jwtService: jwtService,
		logger:     logger,
	}
}

// contextKey is the type for context keys.
type contextKey string

const (
	claimsKey   contextKey = "claims"
	clientIPKey contextKey = "client_ip"
)

// GetClientIP extracts the client IP from context.
func GetClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey).(string)
	return ip
}

// withClientIP returns an http.Handler that injects the client IP into context.
func withClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), clientIPKey, extractRemoteIP(r.RemoteAddr))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isPublicPath reports whether the request path is allowed without JWT auth.
func isPublicPath(path string) bool {
	if path == "/v1/auth/login" {
		return true
	}
	if strings.HasPrefix(path, "/v1/sub/") {
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
			if strings.HasSuffix(r.URL.Path, "/sync/stream") ||
				r.URL.Path == "/v1/ws" ||
				r.URL.Path == "/v1/events/logs" ||
				r.URL.Path == "/v1/connections/stream" ||
				r.URL.Path == "/v1/stats/system/stream" {
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
// through this middleware.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// Flush delegates to the underlying ResponseWriter so SSE streaming works
// through this middleware.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter for http.NewResponseController.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
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

		message := fmt.Sprintf("%s %s %s %d", extractRemoteIP(r.RemoteAddr), r.Method, r.URL.Path, recorder.statusCode)
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
func GetClaims(ctx context.Context) *service.Claims {
	claims, _ := ctx.Value(claimsKey).(*service.Claims)
	return claims
}

// RateLimitMiddleware implements simple IP-based rate limiting.
type RateLimitMiddleware struct {
	logger  *slog.Logger
	limiter *rateLimiter
	stopCh  chan struct{}
	wg      sync.WaitGroup
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
		stopCh:  make(chan struct{}),
	}
	rl.wg.Add(1)
	go rl.cleanupOldEntries()
	return rl
}

// Stop signals the cleanup goroutine to exit and waits for it.
func (m *RateLimitMiddleware) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

func (m *RateLimitMiddleware) cleanupOldEntries() {
	defer m.wg.Done()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.limiter.mu.Lock()
			now := time.Now()
			for ip, state := range m.limiter.ips {
				if len(state.requests) == 0 || now.Sub(state.requests[len(state.requests)-1]) > cleanupInterval {
					delete(m.limiter.ips, ip)
				}
			}
			m.limiter.mu.Unlock()
		case <-m.stopCh:
			return
		}
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
		validIdx := len(state.requests)
		for i, reqTime := range state.requests {
			if now.Sub(reqTime) < time.Minute {
				validIdx = i
				break
			}
		}
		state.requests = state.requests[validIdx:]

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
