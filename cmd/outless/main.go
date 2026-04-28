package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	geoipadapter "outless/internal/adapter/geoip"
	httpadapter "outless/internal/adapter/http"
	"outless/internal/adapter/repository"
	"outless/internal/adapter/xray"
	"outless/internal/domain"
	"outless/internal/migrations"
	"outless/internal/service"
	"outless/shared/config"
	"outless/shared/logging"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
)

// Config bundles runtime settings for unified process.
type Config struct {
	DatabaseURL           string
	HTTPAddress           string
	JWTSecret             string
	JWTExpiry             time.Duration
	ShutdownTimeout       time.Duration
	AdminLogin            string
	AdminPassword         string
	PublicRefreshInterval time.Duration
	RouterPort            int
	RouterDomain          string
	RouterSNI             string
	RouterPrivateKey      string
	RouterPublicKey       string
	RouterShortID         string
	RouterFingerprint     string
	RouterAddress         string
	RouterSyncInterval    time.Duration
	XrayAPIAddress        string
	XrayAPITimeout        time.Duration
	AgentsWorkers         int
	AgentsURL             string
	MonitorGeoIPDBPath    string
	MonitorGeoIPDBURL     string
	MonitorGeoIPAuto      bool
	MonitorGeoIPTTL       time.Duration
	MonitorWorkers        int
	MonitorPollInterval   time.Duration
	MonitorCheckInterval  time.Duration
}

func main() {
	configPath := flag.String("config", "outless.yaml", "path to config file")
	flag.Parse()

	// Create initial logger for config loading errors
	logger := logging.New("outless")

	cfg, yamlCfg, err := loadConfig(*configPath, logger)
	if err != nil {
		logger.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Re-create logger with config-based settings
	logger = logging.NewFromConfig("outless", yamlCfg.Logs, "main")

	// Create separate loggers for different modules
	apiLogger := logging.NewFromConfig("outless", yamlCfg.Logs, "api")
	monitorLogger := logging.NewFromConfig("outless", yamlCfg.Logs, "monitor")
	routerLogger := logging.NewFromConfig("outless", yamlCfg.Logs, "router")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Run database migrations
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("failed to get sql db from gorm", slog.String("error", err.Error()))
		os.Exit(1)
	}
	migrator := migrations.NewMigrator(sqlDB, logger)
	if err := migrator.Up(ctx); err != nil {
		logger.Error("failed to run migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize repositories
	nodeRepo := repository.NewNodeRepository(db, monitorLogger)
	tokenRepo := repository.NewTokenRepository(db, monitorLogger)
	groupRepo := repository.NewGroupRepository(db, monitorLogger)
	publicSourceRepo := repository.NewPublicSourceRepository(db, monitorLogger)
	adminRepo := repository.NewAdminRepository(db, apiLogger)

	// Initialize GeoIP resolver for country detection
	var geoipResolver domain.GeoIPResolver
	if cfg.MonitorGeoIPDBPath != "" {
		resolver, err := geoipadapter.NewMaxMindAdapter(cfg.MonitorGeoIPDBPath, monitorLogger)
		if err != nil {
			logger.Warn("failed to initialize geoip resolver, country detection disabled", slog.String("error", err.Error()))
			geoipResolver = geoipadapter.NewNullGeoIPResolver()
		} else {
			geoipResolver = resolver
		}
	} else {
		geoipResolver = geoipadapter.NewNullGeoIPResolver()
	}

	// Bootstrap admin from env
	if err = bootstrapAdminFromEnv(ctx, adminRepo, cfg, logger); err != nil {
		logger.Error("failed to bootstrap admin from env", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize runtime controller based on configuration.
	var runtime service.RuntimeController
	if cfg.XrayAPIAddress != "" {
		hubConfig := xray.HubInboundConfig{
			Listen:      listenHost(cfg.RouterAddress),
			Port:        cfg.RouterPort,
			SNI:         cfg.RouterSNI,
			PrivateKey:  cfg.RouterPrivateKey,
			ShortID:     cfg.RouterShortID,
			Destination: cfg.RouterSNI + ":443",
		}
		runtime = xray.NewGRPCRuntimeController(routerLogger, tokenRepo, nodeRepo, cfg.XrayAPIAddress, "vless-in", hubConfig)
	}

	// Sync router config for external Xray runtime (no embedded process management).
	hubManager := service.NewRouterManager(tokenRepo, nodeRepo, runtime, service.ManagerConfig{
		ConfigPath:   "/var/lib/outless/xray-hub.json",
		SyncInterval: cfg.RouterSyncInterval,
		Inbound: xray.HubInboundConfig{
			Listen:      listenHost(cfg.RouterAddress),
			Port:        cfg.RouterPort,
			SNI:         cfg.RouterSNI,
			PrivateKey:  cfg.RouterPrivateKey,
			ShortID:     cfg.RouterShortID,
			Destination: cfg.RouterSNI + ":443",
		},
	}, routerLogger)

	// Initialize services
	jwtService := service.NewJWTService(cfg.JWTSecret, cfg.JWTExpiry)
	subscriptionService := service.NewSubscriptionService(nodeRepo, tokenRepo, groupRepo, service.HubConfig{
		Host:         cfg.RouterDomain,
		Port:         cfg.RouterPort,
		SNI:          cfg.RouterSNI,
		PublicKey:    cfg.RouterPublicKey,
		ShortID:      cfg.RouterShortID,
		Fingerprint:  cfg.RouterFingerprint,
		NameTemplate: yamlCfg.Router.NameTemplate,
	}, apiLogger)
	publicService := service.NewPublicService(nodeRepo, publicSourceRepo, groupRepo, geoipResolver, monitorLogger)

	// Initialize HTTP handlers
	realtime := httpadapter.NewRealtimeHandler(
		publicService,
		groupRepo,
		cfg.PublicRefreshInterval,
		filepath.Join(os.TempDir(), "outless", "realtime-state.json"),
		apiLogger,
	)
	handlers := httpadapter.Handlers{
		Subscription: httpadapter.NewSubscriptionHandler(subscriptionService, apiLogger),
		Auth:         httpadapter.NewAuthHandler(adminRepo, jwtService, apiLogger),
		Token:        httpadapter.NewTokenManagementHandler(tokenRepo, groupRepo, apiLogger),
		Node:         httpadapter.NewNodeManagementHandler(nodeRepo, groupRepo, geoipResolver, realtime, apiLogger),
		Group:        httpadapter.NewGroupManagementHandler(groupRepo, nodeRepo, realtime, apiLogger),
		PublicSource: httpadapter.NewPublicSourceManagementHandler(publicSourceRepo, groupRepo, publicService, apiLogger),
		Settings:     httpadapter.NewSettingsHandler(*configPath, apiLogger),
		Admin:        httpadapter.NewAdminManagementHandler(adminRepo, apiLogger),
		Stats:        httpadapter.NewStatsHandler(nodeRepo, tokenRepo, groupRepo, apiLogger),
	}
	server := httpadapter.NewServer(httpadapter.Config{
		Address:           cfg.HTTPAddress,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}, apiLogger, jwtService, realtime, handlers)

	// Start all services in errgroup
	g, gCtx := errgroup.WithContext(ctx)

	// Hub manager
	g.Go(func() error {
		routerLogger.Info("starting hub manager")
		return hubManager.Run(gCtx)
	})

	// HTTP API server
	g.Go(func() error {
		apiLogger.Info("starting http api server", slog.String("address", cfg.HTTPAddress))
		return server.Start()
	})

	// Graceful shutdown
	g.Go(func() error {
		<-gCtx.Done()
		logger.Info("shutting down gracefully")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("http server shutdown failed: %w", shutdownErr)
		}

		if closeErr := geoipResolver.Close(); closeErr != nil {
			logger.Warn("failed to close geoip resolver", slog.String("error", closeErr.Error()))
		}

		return nil
	})

	if err := g.Wait(); err != nil && ctx.Err() == nil {
		logger.Error("unified process exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("unified process shutdown complete")
}

func loadConfig(path string, logger *slog.Logger) (Config, config.Config, error) {
	loader := config.NewLoader(logger)
	yamlCfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(path, &yamlCfg); err != nil {
		return Config{}, config.Config{}, fmt.Errorf("loading config: %w", err)
	}

	if err := yamlCfg.Validate(); err != nil {
		return Config{}, config.Config{}, fmt.Errorf("config validation failed: %w", err)
	}

	cfg := Config{
		DatabaseURL:           yamlCfg.Database.URL,
		HTTPAddress:           ":41220",
		JWTSecret:             yamlCfg.JWT.Secret,
		JWTExpiry:             yamlCfg.JWT.Expiry,
		ShutdownTimeout:       yamlCfg.API.Shutdown,
		AdminLogin:            yamlCfg.Admin.Login,
		AdminPassword:         yamlCfg.Admin.Password,
		PublicRefreshInterval: yamlCfg.Monitor.RefreshInterval,
		RouterPort:            yamlCfg.Router.Port,
		RouterDomain:          yamlCfg.Router.Domain,
		RouterSNI:             yamlCfg.Router.SNI,
		RouterPrivateKey:      yamlCfg.Router.PrivateKey,
		RouterPublicKey:       yamlCfg.Router.PublicKey,
		RouterShortID:         yamlCfg.Router.ShortID,
		RouterFingerprint:     yamlCfg.Router.Fingerprint,
		RouterAddress:         yamlCfg.Router.Address,
		RouterSyncInterval:    yamlCfg.Router.SyncInterval,
		XrayAPIAddress:        yamlCfg.XrayAPI.Address,
		XrayAPITimeout:        yamlCfg.XrayAPI.Timeout,
		AgentsWorkers:         yamlCfg.Monitor.Agents.Workers,
		AgentsURL:             yamlCfg.Monitor.Agents.URL,
		MonitorGeoIPDBPath:    yamlCfg.Monitor.GeoIP.DBPath,
		MonitorGeoIPDBURL:     yamlCfg.Monitor.GeoIP.DBURL,
		MonitorGeoIPAuto:      yamlCfg.Monitor.GeoIP.Auto,
		MonitorGeoIPTTL:       yamlCfg.Monitor.GeoIP.TTL,
		MonitorWorkers:        yamlCfg.Monitor.Workers,
		MonitorPollInterval:   yamlCfg.Monitor.PollInterval,
		MonitorCheckInterval:  yamlCfg.Monitor.CheckInterval,
	}

	if cfg.AgentsWorkers <= 0 {
		cfg.AgentsWorkers = 1
	}

	return cfg, yamlCfg, nil
}

func bootstrapAdminFromEnv(ctx context.Context, adminRepo domain.AdminRepository, cfg Config, logger *slog.Logger) error {
	if cfg.AdminLogin == "" || cfg.AdminPassword == "" {
		logger.Info("admin env bootstrap disabled")
		return nil
	}

	count, err := adminRepo.Count(ctx)
	if err != nil {
		return fmt.Errorf("counting admins before env bootstrap: %w", err)
	}

	if count > 0 {
		logger.Info("admin env bootstrap skipped because admins already exist")
		return nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), 12)
	if err != nil {
		return fmt.Errorf("hashing admin password: %w", err)
	}

	admin := domain.Admin{
		ID:           newAdminID(),
		Username:     cfg.AdminLogin,
		PasswordHash: string(passwordHash),
	}

	if err := adminRepo.Create(ctx, admin); err != nil {
		if errors.Is(err, domain.ErrAdminAlreadyExists) {
			logger.Info("admin env bootstrap skipped due race: first admin already exists")
			return nil
		}
		return fmt.Errorf("creating admin from env: %w", err)
	}

	logger.Info("admin env bootstrap completed", slog.String("username", cfg.AdminLogin))
	return nil
}

func newAdminID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("admin_%d", time.Now().UTC().UnixNano())
	}

	return hex.EncodeToString(bytes)
}

func listenHost(addr string) string {
	if addr == "" {
		return "0.0.0.0"
	}
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			if i == 0 {
				return "0.0.0.0"
			}
			return addr[:i]
		}
	}
	return addr
}
