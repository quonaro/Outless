package httpadapter

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
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

// Wrap returns an http.Handler that validates JWT tokens.
func (m *JWTMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"message":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"message":"invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			m.logger.Warn("invalid token", slog.String("error", err.Error()))
			http.Error(w, `{"message":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggingMiddleware logs all HTTP requests.
type LoggingMiddleware struct {
	logger *slog.Logger
}

// NewLoggingMiddleware constructs a logging middleware.
func NewLoggingMiddleware(logger *slog.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{logger: logger}
}

// Wrap returns an http.Handler that logs requests.
func (m *LoggingMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.logger.Info("request started",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
		)
		next.ServeHTTP(w, r)
		m.logger.Info("request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// GetClaims extracts JWT claims from the request context.
func GetClaims(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(claimsKey).(*auth.Claims)
	return claims
}
