package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"outless/internal/service"
	"outless/web"
)

// Server wraps HTTP subscription API server.
type Server struct {
	server *http.Server
	logger *slog.Logger
}

// Config defines HTTP server settings.
type Config struct {
	Address           string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	DisableDocs       bool
}

// Handlers groups all HTTP handlers the server wires up.
type Handlers struct {
	Subscription *SubscriptionHandler
	Auth         *AuthHandler
	Token        *TokenManagementHandler
	Node         *NodeManagementHandler
	Group        *GroupManagementHandler
	PublicSource *PublicSourceManagementHandler
	Inbound      *InboundManagementHandler
	Settings     *SettingsHandler
	Admin        *AdminManagementHandler
	Stats        *StatsHandler
}

// NewServer builds HTTP server with injected handlers.
func NewServer(cfg Config, logger *slog.Logger, jwtService *service.JWTService, handlers Handlers) *Server {
	apiMux := http.NewServeMux()
	humaCfg := huma.DefaultConfig("Outless API", "0.1.0")
	if cfg.DisableDocs {
		humaCfg.OpenAPIPath = ""
		humaCfg.DocsPath = ""
		humaCfg.SchemasPath = ""
	}
	humaAPI := humago.New(apiMux, humaCfg)
	handlers.Subscription.Register(humaAPI)
	handlers.Auth.Register(humaAPI)
	handlers.Token.Register(humaAPI)
	handlers.Node.Register(humaAPI)
	handlers.Group.Register(humaAPI)
	handlers.PublicSource.Register(humaAPI)
	handlers.Inbound.Register(humaAPI)
	handlers.Settings.Register(humaAPI)
	handlers.Admin.Register(humaAPI)
	handlers.Stats.Register(humaAPI)

	jwtMiddleware := NewJWTMiddleware(jwtService, logger)
	rateLimitMiddleware := NewRateLimitMiddleware(logger)
	loggingMiddleware := NewLoggingMiddleware(logger)

	protectedAPI := jwtMiddleware.Wrap(rateLimitMiddleware.Wrap(apiMux))

	rootMux := http.NewServeMux()
	rootMux.Handle("/v1/", protectedAPI)
	rootMux.Handle("/v1", protectedAPI)

	// The frontend uses /api as baseURL in production, so map /api/v1/ to the
	// same handlers by stripping the /api prefix.
	apiProxy := http.StripPrefix("/api", protectedAPI)
	rootMux.Handle("/api/v1/", apiProxy)
	rootMux.Handle("/api/v1", apiProxy)

	static, err := web.FS()
	if err != nil {
		logger.Error("failed to load embedded frontend", slog.String("error", err.Error()))
	} else {
		fileServer := http.FileServer(static)
		rootMux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Serve static assets (JS, CSS, fonts, images) directly.
			// For directories and non-existent paths, serve 200.html (SPA shell)
			// so client-side routing handles the route instead of pre-rendered
			// meta-refresh redirects.
			if r.URL.Path != "/" {
				if f, openErr := static.Open(r.URL.Path); openErr == nil {
					stat, _ := f.Stat()
					_ = f.Close()
					if !stat.IsDir() {
						fileServer.ServeHTTP(w, r)
						return
					}
				}
			}
			r.URL.Path = "/200.html"
			fileServer.ServeHTTP(w, r)
		}))
	}

	handler := loggingMiddleware.Wrap(rootMux)

	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
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
