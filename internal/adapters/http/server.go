package httpadapter

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"outless/internal/app/auth"

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

// Handlers groups all HTTP handlers the server wires up.
type Handlers struct {
	Subscription *SubscriptionHandler
	Auth         *AuthHandler
	Token        *TokenManagementHandler
	Node         *NodeManagementHandler
	Group        *GroupManagementHandler
	GroupSync    *GroupSyncHandler
	PublicSource *PublicSourceManagementHandler
	Settings     *SettingsHandler
	Admin        *AdminManagementHandler
	Stats        *StatsHandler
}

// NewServer builds HTTP server with injected handlers.
func NewServer(cfg Config, logger *slog.Logger, jwtService *auth.JWTService, handlers Handlers) *Server {
	mux := http.NewServeMux()
	humaAPI := humago.New(mux, huma.DefaultConfig("Outless API", "0.1.0"))
	handlers.Subscription.Register(humaAPI)
	handlers.Auth.Register(humaAPI)
	handlers.Token.Register(humaAPI)
	handlers.Node.Register(humaAPI)
	handlers.Group.Register(humaAPI)
	handlers.PublicSource.Register(humaAPI)
	handlers.Settings.Register(humaAPI)
	handlers.Admin.Register(humaAPI)
	handlers.Stats.Register(humaAPI)

	jwtMiddleware := NewJWTMiddleware(jwtService, logger)
	loggingMiddleware := NewLoggingMiddleware(logger)
	routedHandler := http.Handler(mux)
	if handlers.GroupSync != nil {
		routedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if handlers.GroupSync.HandleStream(w, r) {
				return
			}
			mux.ServeHTTP(w, r)
		})
	}
	baseHandler := jwtMiddleware.Wrap(routedHandler)
	handler := loggingMiddleware.Wrap(baseHandler)

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
