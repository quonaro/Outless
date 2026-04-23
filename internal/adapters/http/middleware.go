package httpadapter

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"

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

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
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
