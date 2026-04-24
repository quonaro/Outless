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
	"outless/internal/app/public"
	"outless/internal/app/subscription"
	"outless/internal/domain"
	"outless/pkg/config"
	"outless/pkg/logging"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
)

// Config defines API process settings.
type Config struct {
	DatabaseURL           string
	HTTPAddress           string
	JWTSecret             string
	JWTExpiry             time.Duration
	ShutdownTimeout       time.Duration
	OutlessLogin          string
	OutlessPassword       string
	PublicRefreshInterval time.Duration
	HubHost               string
	HubPort               int
	HubSNI                string
	HubPublicKey          string
	HubShortID            string
	HubFingerprint        string
	XrayProbeURL          string
	XrayAdminURL          string
	XraySocksAddr         string
	XrayShards            []config.XrayProbeShardConfig
	XrayGeoIPDBPath       string
	XrayGeoIPDBURL        string
	XrayGeoIPAuto         bool
	XrayGeoIPTTL          time.Duration
}

func main() {
	configPath := flag.String("config", "outless.yaml", "path to config file")
	flag.Parse()

	logger := logging.New("api")

	cfg, err := loadConfig(*configPath, logger)
	if err != nil {
		logger.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logXrayStartupStatus(cfg.XrayShards, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.NewGormDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	nodeRepo := postgres.NewGormNodeRepository(db, logger)
	tokenRepo := postgres.NewGormTokenRepository(db, logger)
	groupRepo := postgres.NewGormGroupRepository(db, logger)
	probeJobRepo := postgres.NewGormProbeJobRepository(db, logger)
	publicSourceRepo := postgres.NewGormPublicSourceRepository(db, logger)
	adminRepo := postgres.NewGormAdminRepository(db, logger)
	if err = bootstrapAdminFromEnv(ctx, adminRepo, cfg, logger); err != nil {
		logger.Error("failed to bootstrap admin from env", slog.String("error", err.Error()))
		os.Exit(1)
	}

	jwtService := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiry)
	subscriptionService := subscription.NewService(nodeRepo, tokenRepo, groupRepo, subscription.HubConfig{
		Host:        cfg.HubHost,
		Port:        cfg.HubPort,
		SNI:         cfg.HubSNI,
		PublicKey:   cfg.HubPublicKey,
		ShortID:     cfg.HubShortID,
		Fingerprint: cfg.HubFingerprint,
	}, logger)
	probeEngine, err := xray.NewProbeEnginePool(logger, cfg.XrayProbeURL, cfg.XrayShards, xray.GeoIPConfig{
		DBPath: cfg.XrayGeoIPDBPath,
		DBURL:  cfg.XrayGeoIPDBURL,
		Auto:   cfg.XrayGeoIPAuto,
		TTL:    cfg.XrayGeoIPTTL,
	}, 10*time.Second)
	if err != nil {
		logger.Error("failed to configure xray probe pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("xray probe client configured",
		slog.String("admin_url", cfg.XrayAdminURL),
		slog.String("probe_target", cfg.XrayProbeURL),
		slog.String("socks_addr", cfg.XraySocksAddr),
		slog.String("geoip_db_path", cfg.XrayGeoIPDBPath),
		slog.String("geoip_db_url", cfg.XrayGeoIPDBURL),
		slog.Bool("geoip_auto", cfg.XrayGeoIPAuto),
		slog.Duration("geoip_ttl", cfg.XrayGeoIPTTL),
	)
	publicService := public.NewService(nodeRepo, publicSourceRepo, groupRepo, probeEngine, logger)
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

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return server.Start()
	})

	group.Go(func() error {
		<-groupCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("graceful shutdown failed: %w", shutdownErr)
		}
		return nil
	})

	group.Go(func() error {
		return runPublicSourceWorker(groupCtx, publicService, cfg.PublicRefreshInterval, realtime, logger)
	})

	if err = group.Wait(); err != nil && ctx.Err() == nil {
		logger.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// runPublicSourceWorker triggers ImportAll on startup and every interval tick
// until the context is cancelled.
func runPublicSourceWorker(
	ctx context.Context,
	service *public.Service,
	interval time.Duration,
	realtime *httpadapter.RealtimeHandler,
	logger *slog.Logger,
) error {
	if interval <= 0 {
		logger.Info("public source worker disabled (interval <= 0)")
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
	stateTicker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
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
		DatabaseURL:           yamlCfg.Database.URL,
		HTTPAddress:           ":41220",
		JWTSecret:             yamlCfg.API.JWT.Secret,
		JWTExpiry:             yamlCfg.API.JWT.Expiry,
		ShutdownTimeout:       yamlCfg.API.ShutdownTimeout,
		OutlessLogin:          yamlCfg.API.Admin.Login,
		OutlessPassword:       yamlCfg.API.Admin.Password,
		PublicRefreshInterval: yamlCfg.Checker.PublicRefreshInterval,
		HubHost:               yamlCfg.Hub.Host,
		HubPort:               yamlCfg.Hub.Port,
		HubSNI:                yamlCfg.Hub.SNI,
		HubPublicKey:          yamlCfg.Hub.PublicKey,
		HubShortID:            yamlCfg.Hub.ShortID,
		HubFingerprint:        yamlCfg.Hub.Fingerprint,
		XrayProbeURL:          yamlCfg.Xray.Probe.ProbeURL,
		XrayAdminURL:          yamlCfg.Xray.Probe.AdminURL,
		XraySocksAddr:         yamlCfg.Xray.Probe.SocksAddr,
		XrayShards:            yamlCfg.Xray.Probe.Shards,
		XrayGeoIPDBPath:       yamlCfg.Xray.Probe.GeoIPDBPath,
		XrayGeoIPDBURL:        yamlCfg.Xray.Probe.GeoIPDBURL,
		XrayGeoIPAuto:         yamlCfg.Xray.Probe.GeoIPAuto,
		XrayGeoIPTTL:          yamlCfg.Xray.Probe.GeoIPTTL,
	}
	if err := validateProbeConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateProbeConfig(cfg Config) error {
	if strings.TrimSpace(cfg.XrayAdminURL) == "" {
		return errors.New("xray.probe.admin_url is required")
	}
	if strings.TrimSpace(cfg.XraySocksAddr) == "" {
		return errors.New("xray.probe.socks_addr is required")
	}
	if len(cfg.XrayShards) == 0 {
		return errors.New("xray.probe.shards must contain at least one shard")
	}
	for i := range cfg.XrayShards {
		if strings.TrimSpace(cfg.XrayShards[i].AdminURL) == "" {
			return fmt.Errorf("xray.probe.shards[%d].admin_url is required", i)
		}
		if strings.TrimSpace(cfg.XrayShards[i].SocksAddr) == "" {
			return fmt.Errorf("xray.probe.shards[%d].socks_addr is required", i)
		}
	}
	if strings.TrimSpace(cfg.XrayProbeURL) == "" {
		return errors.New("xray.probe.probe_url is required")
	}
	return nil
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

	// bcrypt cost 12 provides stronger security than DefaultCost (typically 10)
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

func logXrayStartupStatus(shards []config.XrayProbeShardConfig, logger *slog.Logger) {
	for i := range shards {
		checkCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := xray.CheckXrayAPI(checkCtx, shards[i].AdminURL)
		cancel()
		if err != nil {
			logger.Error("Xray probe shard is unavailable",
				slog.Int("shard_index", i),
				slog.String("admin_url", shards[i].AdminURL),
				slog.String("error", err.Error()),
			)
			continue
		}
		logger.Info("Xray probe shard is ready",
			slog.Int("shard_index", i),
			slog.String("admin_url", shards[i].AdminURL),
		)
	}
}
