package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"outless/internal/adapters/docker"
	"outless/internal/adapters/postgres"
	"outless/internal/adapters/xray"
	"outless/internal/app/checker"
	"outless/pkg/config"
	"outless/pkg/logging"

	"golang.org/x/sync/errgroup"
)

func main() {
	configPath := flag.String("config", "outless.yaml", "path to config file")
	flag.Parse()

	logger := logging.New("checker")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig(*configPath, logger)
	if err != nil {
		logger.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logXrayStartupStatus(cfg.XrayShards, logger)

	dockerManager, err := docker.NewContainerManager(logger, cfg.ConfigVolume, cfg.XrayConfigPath)
	if err != nil {
		logger.Error("failed to create docker manager", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dockerManager.Close()

	// Copy config from host to volume if needed
	if err := copyConfigToVolume(cfg.XrayConfigPath, "/host-xray-config.json", logger); err != nil {
		logger.Warn("failed to copy config to volume", slog.String("error", err.Error()))
	}

	db, err := postgres.NewGormDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	repo := postgres.NewGormNodeRepository(db, logger)
	jobRepo := postgres.NewGormProbeJobRepository(db, logger)
	engine, err := xray.NewProbeEnginePool(logger, cfg.ProbeURL, cfg.XrayShards, xray.GeoIPConfig{
		DBPath: cfg.XrayGeoIPDBPath,
		DBURL:  cfg.XrayGeoIPDBURL,
		Auto:   cfg.XrayGeoIPAuto,
		TTL:    cfg.XrayGeoIPTTL,
	}, 10*time.Second)
	if err != nil {
		logger.Error("failed to configure xray probe pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	service := checker.NewService(repo, engine, logger, checker.Config{Workers: cfg.Workers})
	jobRunner := checker.NewJobRunner(jobRepo, repo, engine, logger)

	// Initial sync of probe containers
	if err := syncProbeContainers(ctx, dockerManager, cfg.ShardCount, logger); err != nil {
		logger.Error("initial probe container sync failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	jobTicker := time.NewTicker(cfg.JobPollInterval)
	defer jobTicker.Stop()
	fullCheckTicker := time.NewTicker(cfg.CheckInterval)
	defer fullCheckTicker.Stop()
	probeSyncTicker := time.NewTicker(30 * time.Second)
	defer probeSyncTicker.Stop()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		for {
			select {
			case <-gCtx.Done():
				logger.Info("checker shutting down gracefully")
				return gCtx.Err()
			case <-jobTicker.C:
				if err = jobRunner.RunPending(gCtx, cfg.Workers*2, cfg.Workers); err != nil {
					logger.Error("probe jobs run failed", slog.String("error", err.Error()))
					return err
				}
			case <-fullCheckTicker.C:
				if err = service.RunOnce(gCtx); err != nil {
					logger.Error("checker run failed", slog.String("error", err.Error()))
					return err
				}
			case <-probeSyncTicker.C:
				if err := syncProbeContainers(gCtx, dockerManager, cfg.ShardCount, logger); err != nil {
					logger.Warn("probe container sync failed", slog.String("error", err.Error()))
				}
			}
		}
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("checker exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Cleanup probe containers on exit
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := cleanupProbeContainers(cleanupCtx, dockerManager, logger); err != nil {
		logger.Error("probe container cleanup failed", slog.String("error", err.Error()))
	}

	logger.Info("checker shutdown complete")
}

// Config defines checker process settings.
type Config struct {
	DatabaseURL     string
	Workers         int
	ProbeURL        string
	XrayAdminURL    string
	XraySocksAddr   string
	XrayShards      []config.XrayProbeShardConfig
	XrayGeoIPDBPath string
	XrayGeoIPDBURL  string
	XrayGeoIPAuto   bool
	XrayGeoIPTTL    time.Duration
	JobPollInterval time.Duration
	CheckInterval   time.Duration
	ShardCount      int
	XrayConfigPath  string
	ConfigVolume    string
}

func loadConfig(path string, logger *slog.Logger) (Config, error) {
	loader := config.NewLoader(logger)
	yamlCfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(path, &yamlCfg); err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}

	cfg := Config{
		DatabaseURL:     yamlCfg.Database.URL,
		Workers:         yamlCfg.Checker.Workers,
		ProbeURL:        yamlCfg.Xray.Probe.ProbeURL,
		XrayAdminURL:    yamlCfg.Xray.Probe.AdminURL,
		XraySocksAddr:   yamlCfg.Xray.Probe.SocksAddr,
		XrayShards:      yamlCfg.Xray.Probe.Shards,
		XrayGeoIPDBPath: yamlCfg.Xray.Probe.GeoIPDBPath,
		XrayGeoIPDBURL:  yamlCfg.Xray.Probe.GeoIPDBURL,
		XrayGeoIPAuto:   yamlCfg.Xray.Probe.GeoIPAuto,
		XrayGeoIPTTL:    yamlCfg.Xray.Probe.GeoIPTTL,
		JobPollInterval: yamlCfg.Checker.JobPollInterval,
		CheckInterval:   yamlCfg.Checker.CheckInterval,
		ShardCount:      yamlCfg.Xray.Probe.ShardCount,
		XrayConfigPath:  os.Getenv("XRAY_CONFIG_PATH"),
		ConfigVolume:    os.Getenv("XRAY_CONFIG_VOLUME"),
	}

	if cfg.XrayConfigPath == "" {
		cfg.XrayConfigPath = "/app/xray/config.json"
	}
	if cfg.ConfigVolume == "" {
		cfg.ConfigVolume = "xray_config"
	}
	if err := validateProbeConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateProbeConfig(cfg Config) error {
	if strings.TrimSpace(cfg.ProbeURL) == "" {
		return errors.New("xray.probe.probe_url is required")
	}
	if cfg.JobPollInterval <= 0 {
		return errors.New("checker.job_poll_interval must be greater than 0")
	}
	if cfg.CheckInterval <= 0 {
		return errors.New("checker.check_interval must be greater than 0")
	}
	if cfg.ShardCount <= 0 {
		return errors.New("xray.probe.shard_count must be greater than 0")
	}
	if len(cfg.XrayShards) == 0 {
		return errors.New("xray.probe.shards must contain at least one shard (or set shard_count)")
	}
	for i := range cfg.XrayShards {
		if strings.TrimSpace(cfg.XrayShards[i].AdminURL) == "" {
			return fmt.Errorf("xray.probe.shards[%d].admin_url is required", i)
		}
		if strings.TrimSpace(cfg.XrayShards[i].SocksAddr) == "" {
			return fmt.Errorf("xray.probe.shards[%d].socks_addr is required", i)
		}
	}
	return nil
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

// copyConfigToVolume copies the Xray config from host to the volume.
func copyConfigToVolume(volumePath, hostPath string, logger *slog.Logger) error {
	src, err := os.ReadFile(hostPath)
	if err != nil {
		return fmt.Errorf("reading host config: %w", err)
	}

	if err := os.WriteFile(volumePath, src, 0644); err != nil {
		return fmt.Errorf("writing config to volume: %w", err)
	}

	logger.Info("config copied to volume", slog.String("volume_path", volumePath))
	return nil
}

// syncProbeContainers ensures the correct number of probe containers exist.
func syncProbeContainers(ctx context.Context, dockerMgr *docker.ContainerManager, shardCount int, logger *slog.Logger) error {
	existing, err := dockerMgr.ListProbeContainers(ctx)
	if err != nil {
		return fmt.Errorf("listing existing containers: %w", err)
	}

	existingMap := make(map[string]bool)
	for _, name := range existing {
		existingMap[name] = true
	}

	// Create missing containers
	for i := 1; i <= shardCount; i++ {
		name := fmt.Sprintf("xray-probe-%d", i)
		if !existingMap[name] {
			if err := dockerMgr.CreateProbeContainer(ctx, name); err != nil {
				return fmt.Errorf("creating container %s: %w", name, err)
			}
		}
	}

	// Remove excess containers
	for _, name := range existing {
		shardNum := 0
		if _, err := fmt.Sscanf(name, "xray-probe-%d", &shardNum); err == nil {
			if shardNum > shardCount {
				if err := dockerMgr.RemoveProbeContainer(ctx, name); err != nil {
					return fmt.Errorf("removing container %s: %w", name, err)
				}
			}
		}
	}

	logger.Info("probe containers synced", slog.Int("desired", shardCount), slog.Int("existing", len(existing)))
	return nil
}

// cleanupProbeContainers removes all managed probe containers.
func cleanupProbeContainers(ctx context.Context, dockerMgr *docker.ContainerManager, logger *slog.Logger) error {
	logger.Info("cleaning up probe containers")
	existing, err := dockerMgr.ListProbeContainers(ctx)
	if err != nil {
		return fmt.Errorf("listing containers for cleanup: %w", err)
	}

	for _, name := range existing {
		if err := dockerMgr.RemoveProbeContainer(ctx, name); err != nil {
			logger.Warn("failed to remove container during cleanup",
				slog.String("name", name),
				slog.String("error", err.Error()))
		}
	}

	return nil
}
