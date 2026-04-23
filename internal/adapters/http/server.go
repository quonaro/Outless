package httpadapter

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// Server wraps HTTP subscription API server.
type Server struct {
	server *http.Server
	logger *slog.Logger
}

// Config defines HTTP server settings.
type Config struct {
	Address string
}

// NewServer builds HTTP server with injected handlers.
func NewServer(cfg Config, logger *slog.Logger, subscriptionHandler *SubscriptionHandler, authHandler *AuthHandler) *Server {
	mux := http.NewServeMux()
	humaAPI := humago.New(mux, huma.DefaultConfig("Outless API", "0.1.0"))
	subscriptionHandler.Register(humaAPI)
	authHandler.Register(humaAPI)

	loggingMiddleware := NewLoggingMiddleware(logger)
	handler := loggingMiddleware.Wrap(mux)

	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Server{server: srv, logger: logger}
}

// Start launches the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("http server starting", slog.String("addr", s.server.Addr))
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("starting http server: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutting down http server: %w", err)
	}
	s.logger.Info("http server stopped")
	return nil
}
