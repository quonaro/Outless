package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	httpadapter "outless/internal/adapter/http"
	"outless/internal/adapter/repository"
	"outless/internal/adapter/singbox"
	"outless/internal/domain"
	"outless/internal/service"
	"outless/internal/utils"
	"outless/shared/config"
	"outless/shared/logging"

	"github.com/quonaro/lota/engine"

	"golang.org/x/crypto/bcrypt"
)

//go:embed cli.yml
var cliYAML []byte

const defaultConfigPath = "config.yaml"

func main() {
	builder := engine.NewBuilder("outless", cliYAML)
	builder.RegisterNative("run", runServer)
	builder.RegisterNative("reset-password", resetAdminPassword)
	builder.RegisterNative("version", showVersion)
	builder.RegisterNative("show", showConfig)
	builder.RegisterNative("validate", validateConfig)
	builder.RegisterNative("backup", backupDB)
	builder.RegisterNative("status", checkStatus)

	app, err := builder.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		app.PrintHelp()
		return
	}

	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		var groupErr *engine.GroupError
		if errors.As(err, &groupErr) {
			app.PrintGroupHelp(groupErr.Groups)
			return
		}
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		os.Exit(1)
	}
}

//nolint:funlen
func runServer(ctx context.Context, nctx engine.NativeContext) error {
	cfgPath := os.Getenv("OUTLESS_CONFIG")
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}

	logger := logging.New("outless")
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	loader := config.NewLoader(logger)
	cfg := config.DefaultConfig()
	if err := loader.LoadOrCreate(cfgPath, &cfg); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	logger = logging.NewFromConfig("outless", cfg.App.LogLevel, "")

	broadcaster := httpadapter.NewLogBroadcaster()
	logger = slog.New(httpadapter.NewBroadcastHandler(logger.Handler(), broadcaster))

	db, err := repository.NewDB(string(cfg.Database))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	// Repositories
	nodeRepo := repository.NewNodeRepository(db, logger)
	tokenRepo := repository.NewTokenRepository(db, logger)
	groupRepo := repository.NewGroupRepository(db, logger)
	publicSourceRepo := repository.NewPublicSourceRepository(db, logger)
	adminRepo := repository.NewAdminRepository(db, logger)
	inboundRepo := repository.NewInboundRepository(db, logger)

	// Ensure default admin exists if no admins are registered.
	if err := ensureDefaultAdmin(ctx, adminRepo, logger); err != nil {
		return err
	}

	// Services
	jwtService, err := service.NewJWTService(cfg.JWT.Secret, cfg.JWT.Expiry)
	if err != nil {
		return fmt.Errorf("creating jwt service: %w", err)
	}
	publicService := service.NewPublicService(nodeRepo, publicSourceRepo, groupRepo, logger)
	subscriptionService := service.NewSubscriptionService(nodeRepo, tokenRepo, groupRepo, inboundRepo, cfg.App.ExternalHost, logger)
	totpService := service.NewTOTPService()

	// Runtime controller (embedded sing-box)
	runtime := singbox.NewRuntimeController(logger, tokenRepo, nodeRepo, inboundRepo, cfg.App.SingboxLogLevel, 0, broadcaster.Broadcast)
	logger.Info("using embedded sing-box runtime")

	trafficRepo := repository.NewTrafficRepository(db)
	trafficCollector := service.NewTrafficCollector(runtime, trafficRepo, tokenRepo, logger)
	cleanupService := service.NewCleanupService(tokenRepo, logger).WithTrafficRepo(trafficRepo)

	// HTTP handlers
	handlers := httpadapter.Handlers{
		Subscription:      httpadapter.NewSubscriptionHandler(subscriptionService, tokenRepo, logger),
		Auth:              httpadapter.NewAuthHandler(adminRepo, jwtService, totpService, logger),
		Token:             httpadapter.NewTokenManagementHandler(tokenRepo, groupRepo, nodeRepo, inboundRepo, runtime, logger),
		Node:              httpadapter.NewNodeManagementHandler(nodeRepo, groupRepo, logger),
		Group:             httpadapter.NewGroupManagementHandler(groupRepo, nodeRepo, subscriptionService, logger),
		PublicSource:      httpadapter.NewPublicSourceManagementHandler(publicSourceRepo, groupRepo, publicService, logger),
		Inbound:           httpadapter.NewInboundManagementHandler(inboundRepo, logger),
		Settings:          httpadapter.NewSettingsHandler(cfgPath, logger),
		Admin:             httpadapter.NewAdminManagementHandler(adminRepo, logger),
		Stats:             httpadapter.NewStatsHandler(nodeRepo, tokenRepo, groupRepo, inboundRepo, trafficRepo, logger),
		System:            httpadapter.NewSystemMetricsHandler(runtime, logger),
		Traffic:           httpadapter.NewTrafficHandler(trafficRepo, tokenRepo, logger),
		Connections:       httpadapter.NewConnectionsHandler(runtime, logger),
		StreamConnections: httpadapter.NewStreamConnectionsHandler(runtime, logger),
		ImportExport:      httpadapter.NewImportExportHandler(nodeRepo, tokenRepo, groupRepo, publicSourceRepo, inboundRepo, logger),
		LogStream:         httpadapter.NewLogStreamHandler(broadcaster),
	}

	httpConfig := httpadapter.Config{
		Address:           fmt.Sprintf("0.0.0.0:%d", cfg.App.HTTPPort),
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		DisableDocs:       cfg.App.DisableDocs,
	}
	server := httpadapter.NewServer(httpConfig, logger, jwtService, handlers)

	routerManager := service.NewRouterManager(runtime, 0, logger)

	// Start background services
	if err := cleanupService.Start(ctx); err != nil {
		return fmt.Errorf("starting cleanup service: %w", err)
	}
	defer func() {
		_ = cleanupService.Stop()
	}()

	if err := trafficCollector.Start(ctx); err != nil {
		return fmt.Errorf("starting traffic collector: %w", err)
	}
	defer func() {
		_ = trafficCollector.Stop()
	}()

	go func() {
		if err := routerManager.Run(ctx); err != nil {
			logger.Error("router manager stopped", slog.String("error", err.Error()))
			stop()
		}
	}()

	go func() {
		if err := server.Start(); err != nil {
			logger.Error("http server stopped", slog.String("error", err.Error()))
			stop()
		}
	}()

	logger.Info("outless started", slog.String("addr", httpConfig.Address), slog.String("config", cfgPath))

	<-ctx.Done()

	logger.Info("shutdown signal received, initiating graceful shutdown", slog.Duration("gracetime", cfg.App.ShutdownGracetime))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownGracetime)
	defer cancel()

	logger.Info("stopping http server...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", slog.String("error", err.Error()))
	}

	logger.Info("stopping sing-box runtime...")
	runtime.Stop()

	logger.Info("outless stopped")
	return nil
}

// ensureDefaultAdmin creates a default admin/admin account if no admins exist.
func ensureDefaultAdmin(ctx context.Context, adminRepo domain.AdminRepository, logger *slog.Logger) error {
	count, err := adminRepo.Count(ctx)
	if err != nil {
		return fmt.Errorf("counting admins: %w", err)
	}
	if count > 0 {
		return nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin"), 12)
	if err != nil {
		return fmt.Errorf("hashing default admin password: %w", err)
	}

	admin := domain.Admin{
		ID:           utils.NewAdminID(),
		Username:     "admin",
		PasswordHash: string(passwordHash),
	}
	if err := adminRepo.Create(ctx, admin); err != nil {
		return fmt.Errorf("creating default admin: %w", err)
	}

	logger.Info("default admin created", slog.String("username", admin.Username))
	return nil
}

func resetAdminPassword(ctx context.Context, nctx engine.NativeContext) error {
	cfgPath := nctx.Vars["CONFIG_PATH"]
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}

	logger := logging.New("outless")
	loader := config.NewLoader(logger)
	cfg := config.DefaultConfig()
	if err := loader.LoadOrCreate(cfgPath, &cfg); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	db, err := repository.NewDB(string(cfg.Database))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	adminRepo := repository.NewAdminRepository(db, logger)

	username := nctx.Args["username"]
	password := nctx.Args["password"]
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	admin, err := adminRepo.FindByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("finding admin: %w", err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	admin.PasswordHash = string(passwordHash)
	if err := adminRepo.Update(ctx, admin); err != nil {
		return fmt.Errorf("updating admin: %w", err)
	}

	_, _ = fmt.Fprintf(nctx.Stdout, "Password reset for admin %q\n", username)
	return nil
}

func showVersion(_ context.Context, nctx engine.NativeContext) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		_, _ = fmt.Fprintln(nctx.Stdout, "outless version unknown")
		return nil
	}
	_, _ = fmt.Fprintf(nctx.Stdout, "outless version %s\n", info.Main.Version)
	return nil
}
