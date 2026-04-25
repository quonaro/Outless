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

	httpadapter "outless/internal/adapters/http"
	"outless/internal/adapters/postgres"
	"outless/internal/adapters/xray"
	"outless/internal/app/auth"
	"outless/internal/app/monitor"
	"outless/internal/app/public"
	"outless/internal/app/router"
	"outless/internal/app/subscription"
	"outless/internal/domain"
	"outless/internal/migrations"
	"outless/pkg/config"
	"outless/pkg/logging"

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
	agentLogger := logging.NewFromConfig("outless", yamlCfg.Logs, "agent")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.NewGormDB(cfg.DatabaseURL)
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
	nodeRepo := postgres.NewGormNodeRepository(db, monitorLogger)
	tokenRepo := postgres.NewGormTokenRepository(db, monitorLogger)
	groupRepo := postgres.NewGormGroupRepository(db, monitorLogger)
	probeJobRepo := postgres.NewGormProbeJobRepository(db, monitorLogger)
	publicSourceRepo := postgres.NewGormPublicSourceRepository(db, monitorLogger)
	adminRepo := postgres.NewGormAdminRepository(db, apiLogger)

	// Bootstrap admin from env
	if err = bootstrapAdminFromEnv(ctx, adminRepo, cfg, logger); err != nil {
		logger.Error("failed to bootstrap admin from env", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Start embedded Xray edge
	hubRuntime := xray.NewEmbeddedHubRuntime(routerLogger, "xray", "/var/lib/outless/xray-hub.json", yamlCfg.Logs.XrayFilePath, yamlCfg.Logs.Rotation)
	hubManager := router.NewManager(tokenRepo, nodeRepo, hubRuntime, router.ManagerConfig{
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

	// Start embedded probe pool
	probePool := xray.NewProbeRuntimePool(agentLogger, "xray", 10085, "/tmp/outless-probe", yamlCfg.Logs.XrayFilePath, yamlCfg.Logs.Rotation)
	if err := probePool.Start(ctx, cfg.AgentsWorkers); err != nil {
		logger.Error("failed to start embedded probe pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer probePool.Stop()

	probeShards := probePool.ShardConfigs()

	// Initialize probe engine
	probeEngine, err := xray.NewProbeEnginePool(agentLogger, cfg.AgentsURL, probeShards, xray.GeoIPConfig{
		DBPath: cfg.MonitorGeoIPDBPath,
		DBURL:  cfg.MonitorGeoIPDBURL,
		Auto:   cfg.MonitorGeoIPAuto,
		TTL:    cfg.MonitorGeoIPTTL,
	}, 10*time.Second)
	if err != nil {
		logger.Error("failed to configure xray probe pool", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize services
	jwtService := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiry)
	subscriptionService := subscription.NewService(nodeRepo, tokenRepo, groupRepo, subscription.HubConfig{
		Host:         cfg.RouterDomain,
		Port:         cfg.RouterPort,
		SNI:          cfg.RouterSNI,
		PublicKey:    cfg.RouterPublicKey,
		ShortID:      cfg.RouterShortID,
		Fingerprint:  cfg.RouterFingerprint,
		NameTemplate: yamlCfg.Router.NameTemplate,
	}, apiLogger)
	publicService := public.NewService(nodeRepo, publicSourceRepo, groupRepo, probeEngine, monitorLogger)

	// Initialize monitor service
	monitorService := monitor.NewService(nodeRepo, probeEngine, monitorLogger, monitor.Config{Workers: cfg.MonitorWorkers})
	jobRunner := monitor.NewJobRunner(probeJobRepo, nodeRepo, probeEngine, monitorLogger)

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
		Node:         httpadapter.NewNodeManagementHandler(nodeRepo, groupRepo, probeJobRepo, realtime, apiLogger),
		Group:        httpadapter.NewGroupManagementHandler(groupRepo, nodeRepo, probeJobRepo, realtime, apiLogger),
		ProbeJobs:    httpadapter.NewProbeJobHandler(probeJobRepo, apiLogger),
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

	// Checker worker
	g.Go(func() error {
		monitorLogger.Info("starting monitor worker")
		return runMonitorWorker(gCtx, monitorService, jobRunner, cfg.MonitorPollInterval, cfg.MonitorCheckInterval, cfg.MonitorWorkers, monitorLogger)
	})

	// Public source worker
	g.Go(func() error {
		monitorLogger.Info("starting public source worker")
		return runPublicSourceWorker(gCtx, publicService, cfg.PublicRefreshInterval, realtime, monitorLogger)
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

		return nil
	})

	if err := g.Wait(); err != nil && ctx.Err() == nil {
		logger.Error("unified process exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("unified process shutdown complete")
}

// runMonitorWorker runs the monitor service with periodic checks.
func runMonitorWorker(
	ctx context.Context,
	service *monitor.Service,
	jobRunner *monitor.JobRunner,
	jobPollInterval, checkInterval time.Duration,
	workers int,
	logger *slog.Logger,
) error {
	jobTicker := time.NewTicker(jobPollInterval)
	defer jobTicker.Stop()
	fullCheckTicker := time.NewTicker(checkInterval)
	defer fullCheckTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("monitor worker shutting down")
			return nil
		case <-jobTicker.C:
			if err := jobRunner.RunPending(ctx, workers*2, workers); err != nil {
				logger.Error("probe jobs run failed", slog.String("error", err.Error()))
				return err
			}
		case <-fullCheckTicker.C:
			if err := service.RunOnce(ctx); err != nil {
				logger.Error("checker run failed", slog.String("error", err.Error()))
				return err
			}
		}
	}
}

// runPublicSourceWorker triggers ImportAll on startup and every interval.
func runPublicSourceWorker(
	ctx context.Context,
	service *public.Service,
	interval time.Duration,
	realtime *httpadapter.RealtimeHandler,
	logger *slog.Logger,
) error {
	if interval <= 0 {
		logger.Info("public source worker disabled")
		realtime.UpdatePublicRefreshSchedule(nil, nil)
		<-ctx.Done()
		return nil
	}

	logger.Info("public source worker started", slog.Duration("interval", interval))

	nextRunAt := time.Now().UTC().Add(interval)

	runOnce := func() {
		runStartedAt := time.Now().UTC()
		nextRunAt = runStartedAt.Add(interval)
		realtime.UpdatePublicRefreshSchedule(&runStartedAt, &nextRunAt)
		if err := service.ImportAll(ctx); err != nil {
			logger.Warn("public source import failed", slog.String("error", err.Error()))
		}
	}

	realtime.UpdatePublicRefreshSchedule(nil, &nextRunAt)
	runOnce()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	stateTicker := time.NewTicker(1 * time.Minute)
	defer stateTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("public source worker stopped")
			return nil
		case <-ticker.C:
			runOnce()
		case <-stateTicker.C:
			realtime.UpdatePublicRefreshSchedule(nil, &nextRunAt)
		}
	}
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
