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
	"strings"
	"syscall"
	"time"

	httpadapter "outless/internal/adapters/http"
	"outless/internal/adapters/postgres"
	"outless/internal/adapters/xray"
	"outless/internal/app/auth"
	"outless/internal/app/checker"
	"outless/internal/app/hub"
	"outless/internal/app/public"
	"outless/internal/app/subscription"
	"outless/internal/domain"
	"outless/pkg/config"
	"outless/pkg/logging"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
)

// Config bundles runtime settings for unified process.
type Config struct {
	DatabaseURL            string
	HTTPAddress            string
	JWTSecret              string
	JWTExpiry              time.Duration
	ShutdownTimeout        time.Duration
	OutlessLogin           string
	OutlessPassword        string
	PublicRefreshInterval  time.Duration
	HubHost                string
	HubPort                int
	HubSNI                 string
	HubPrivateKey          string
	HubPublicKey           string
	HubShortID             string
	HubFingerprint         string
	HubListenAddress       string
	HubConfigPath          string
	HubXrayBinary          string
	HubSyncInterval        time.Duration
	HubRuntimeMode         config.XrayRuntimeMode
	ProbeURL               string
	ProbeShardCount        int
	ProbeXrayBinary        string
	ProbeGeoIPDBPath       string
	ProbeGeoIPDBURL        string
	ProbeGeoIPAuto         bool
	ProbeGeoIPTTL          time.Duration
	CheckerWorkers         int
	CheckerJobPollInterval time.Duration
	CheckerCheckInterval   time.Duration
}

func main() {
	configPath := flag.String("config", "outless.yaml", "path to config file")
	flag.Parse()

	logger := logging.New("unified")

	cfg, err := loadConfig(*configPath, logger)
	if err != nil {
		logger.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.NewGormDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize repositories
	nodeRepo := postgres.NewGormNodeRepository(db, logger)
	tokenRepo := postgres.NewGormTokenRepository(db, logger)
	groupRepo := postgres.NewGormGroupRepository(db, logger)
	probeJobRepo := postgres.NewGormProbeJobRepository(db, logger)
	publicSourceRepo := postgres.NewGormPublicSourceRepository(db, logger)
	adminRepo := postgres.NewGormAdminRepository(db, logger)

	// Bootstrap admin from env
	if err = bootstrapAdminFromEnv(ctx, adminRepo, cfg, logger); err != nil {
		logger.Error("failed to bootstrap admin from env", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Build hub runtime controller
	hubRuntime, err := buildHubRuntimeController(cfg, logger)
	if err != nil {
		logger.Error("invalid hub runtime config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Start embedded Xray edge
	hubManager := hub.NewManager(tokenRepo, nodeRepo, hubRuntime, hub.ManagerConfig{
		ConfigPath:   "/app/tmp/xray-hub.json",
		SyncInterval: cfg.HubSyncInterval,
		Inbound: xray.HubInboundConfig{
			Listen:      listenHost(cfg.HubListenAddress),
			Port:        cfg.HubPort,
			SNI:         cfg.HubSNI,
			PrivateKey:  cfg.HubPrivateKey,
			ShortID:     cfg.HubShortID,
			Destination: cfg.HubSNI + ":443",
		},
	}, logger)

	// Start embedded probe pool
	probePool := xray.NewProbeRuntimePool(logger, cfg.ProbeXrayBinary, 10085, "/tmp/outless-probe")
	if err := probePool.Start(ctx, cfg.ProbeShardCount); err != nil {
		logger.Error("failed to start embedded probe pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer probePool.Stop()

	probeShards := probePool.ShardConfigs()

	// Initialize probe engine
	probeEngine, err := xray.NewProbeEnginePool(logger, cfg.ProbeURL, probeShards, xray.GeoIPConfig{
		DBPath: cfg.ProbeGeoIPDBPath,
		DBURL:  cfg.ProbeGeoIPDBURL,
		Auto:   cfg.ProbeGeoIPAuto,
		TTL:    cfg.ProbeGeoIPTTL,
	}, 10*time.Second)
	if err != nil {
		logger.Error("failed to configure xray probe pool", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize services
	jwtService := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiry)
	subscriptionService := subscription.NewService(nodeRepo, tokenRepo, groupRepo, subscription.HubConfig{
		Host:        cfg.HubHost,
		Port:        cfg.HubPort,
		SNI:         cfg.HubSNI,
		PublicKey:   cfg.HubPublicKey,
		ShortID:     cfg.HubShortID,
		Fingerprint: cfg.HubFingerprint,
	}, logger)
	publicService := public.NewService(nodeRepo, publicSourceRepo, groupRepo, probeEngine, logger)

	// Initialize checker service
	checkerService := checker.NewService(nodeRepo, probeEngine, logger, checker.Config{Workers: cfg.CheckerWorkers})
	jobRunner := checker.NewJobRunner(probeJobRepo, nodeRepo, probeEngine, logger)

	// Initialize HTTP handlers
	realtime := httpadapter.NewRealtimeHandler(
		publicService,
		groupRepo,
		cfg.PublicRefreshInterval,
		filepath.Join(os.TempDir(), "outless", "realtime-state.json"),
		logger,
	)
	handlers := httpadapter.Handlers{
		Subscription: httpadapter.NewSubscriptionHandler(subscriptionService, logger),
		Auth:         httpadapter.NewAuthHandler(adminRepo, jwtService, logger),
		Token:        httpadapter.NewTokenManagementHandler(tokenRepo, groupRepo, logger),
		Node:         httpadapter.NewNodeManagementHandler(nodeRepo, groupRepo, probeJobRepo, realtime, logger),
		Group:        httpadapter.NewGroupManagementHandler(groupRepo, nodeRepo, probeJobRepo, realtime, logger),
		ProbeJobs:    httpadapter.NewProbeJobHandler(probeJobRepo, logger),
		PublicSource: httpadapter.NewPublicSourceManagementHandler(publicSourceRepo, groupRepo, publicService, logger),
		Settings:     httpadapter.NewSettingsHandler(*configPath, logger),
		Admin:        httpadapter.NewAdminManagementHandler(adminRepo, logger),
		Stats:        httpadapter.NewStatsHandler(nodeRepo, tokenRepo, groupRepo, logger),
	}
	server := httpadapter.NewServer(httpadapter.Config{
		Address:           cfg.HTTPAddress,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}, logger, jwtService, realtime, handlers)

	// Start all services in errgroup
	g, gCtx := errgroup.WithContext(ctx)

	// Hub manager
	g.Go(func() error {
		logger.Info("starting hub manager")
		return hubManager.Run(gCtx)
	})

	// HTTP API server
	g.Go(func() error {
		logger.Info("starting http api server", slog.String("address", cfg.HTTPAddress))
		return server.Start()
	})

	// Checker worker
	g.Go(func() error {
		logger.Info("starting checker worker")
		return runCheckerWorker(gCtx, checkerService, jobRunner, cfg.CheckerJobPollInterval, cfg.CheckerCheckInterval, cfg.CheckerWorkers, logger)
	})

	// Public source worker
	g.Go(func() error {
		logger.Info("starting public source worker")
		return runPublicSourceWorker(gCtx, publicService, cfg.PublicRefreshInterval, realtime, logger)
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

		hubRuntime.Stop()
		return nil
	})

	if err := g.Wait(); err != nil && ctx.Err() == nil {
		logger.Error("unified process exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("unified process shutdown complete")
}

// runCheckerWorker runs the checker service with periodic checks.
func runCheckerWorker(
	ctx context.Context,
	service *checker.Service,
	jobRunner *checker.JobRunner,
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
			logger.Info("checker worker shutting down")
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

func loadConfig(path string, logger *slog.Logger) (Config, error) {
	loader := config.NewLoader(logger)
	yamlCfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(path, &yamlCfg); err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}

	yamlCfg.ApplyCompatibility()

	if err := yamlCfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("config validation failed: %w", err)
	}

	cfg := Config{
		DatabaseURL:            yamlCfg.Database.URL,
		HTTPAddress:            ":41220",
		JWTSecret:              yamlCfg.API.JWT.Secret,
		JWTExpiry:              yamlCfg.API.JWT.Expiry,
		ShutdownTimeout:        yamlCfg.API.ShutdownTimeout,
		OutlessLogin:           yamlCfg.API.Admin.Login,
		OutlessPassword:        yamlCfg.API.Admin.Password,
		PublicRefreshInterval:  yamlCfg.Checker.PublicRefreshInterval,
		HubHost:                yamlCfg.Hub.Host,
		HubPort:                yamlCfg.Hub.Port,
		HubSNI:                 yamlCfg.Hub.SNI,
		HubPrivateKey:          yamlCfg.Hub.PrivateKey,
		HubPublicKey:           yamlCfg.Hub.PublicKey,
		HubShortID:             yamlCfg.Hub.ShortID,
		HubFingerprint:         yamlCfg.Hub.Fingerprint,
		HubListenAddress:       yamlCfg.Hub.ListenAddress,
		HubConfigPath:          yamlCfg.Xray.Edge.ConfigPath,
		HubXrayBinary:          yamlCfg.Xray.Edge.XrayBinary,
		HubSyncInterval:        yamlCfg.Hub.SyncInterval,
		HubRuntimeMode:         yamlCfg.Xray.Edge.RuntimeMode,
		ProbeURL:               yamlCfg.Xray.Probe.ProbeURL,
		ProbeShardCount:        yamlCfg.Xray.Probe.ShardCount,
		ProbeXrayBinary:        yamlCfg.Xray.Probe.XrayBinary,
		ProbeGeoIPDBPath:       yamlCfg.Xray.Probe.GeoIPDBPath,
		ProbeGeoIPDBURL:        yamlCfg.Xray.Probe.GeoIPDBURL,
		ProbeGeoIPAuto:         yamlCfg.Xray.Probe.GeoIPAuto,
		ProbeGeoIPTTL:          yamlCfg.Xray.Probe.GeoIPTTL,
		CheckerWorkers:         yamlCfg.Checker.Workers,
		CheckerJobPollInterval: yamlCfg.Checker.JobPollInterval,
		CheckerCheckInterval:   yamlCfg.Checker.CheckInterval,
	}

	if cfg.ProbeShardCount <= 0 {
		cfg.ProbeShardCount = 1
	}

	return cfg, nil
}

func buildHubRuntimeController(cfg Config, logger *slog.Logger) (hub.RuntimeController, error) {
	if strings.TrimSpace(cfg.HubSNI) == "" {
		return nil, errors.New("hub.sni is required")
	}
	if strings.TrimSpace(cfg.HubPublicKey) == "" {
		return nil, errors.New("hub.public_key is required")
	}
	if strings.TrimSpace(cfg.HubConfigPath) == "" {
		return nil, errors.New("xray.edge.config_path is required")
	}

	switch cfg.HubRuntimeMode {
	case config.XrayRuntimeEmbedded:
		if strings.TrimSpace(cfg.HubXrayBinary) == "" {
			return nil, errors.New("xray.edge.xray_binary is required in embedded mode")
		}
		return xray.NewEmbeddedRuntimeController(logger, cfg.HubXrayBinary), nil
	case config.XrayRuntimeExternal:
		return xray.NewDockerRuntimeController(logger, "outless-xray-edge"), nil
	default:
		return nil, fmt.Errorf("unsupported xray.edge.runtime_mode: %q", cfg.HubRuntimeMode)
	}
}

func bootstrapAdminFromEnv(ctx context.Context, adminRepo domain.AdminRepository, cfg Config, logger *slog.Logger) error {
	if cfg.OutlessLogin == "" || cfg.OutlessPassword == "" {
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

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(cfg.OutlessPassword), 12)
	if err != nil {
		return fmt.Errorf("hashing OUTLESS_PASSWORD: %w", err)
	}

	admin := domain.Admin{
		ID:           newAdminID(),
		Username:     cfg.OutlessLogin,
		PasswordHash: string(passwordHash),
	}

	if err := adminRepo.Create(ctx, admin); err != nil {
		if errors.Is(err, domain.ErrAdminAlreadyExists) {
			logger.Info("admin env bootstrap skipped due race: first admin already exists")
			return nil
		}
		return fmt.Errorf("creating admin from env: %w", err)
	}

	logger.Info("admin env bootstrap completed", slog.String("username", cfg.OutlessLogin))
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
