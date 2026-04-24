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

	"outless/internal/adapters/postgres"
	"outless/internal/adapters/xray"
	"outless/internal/app/checker"
	"outless/pkg/config"
	"outless/pkg/logging"
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

	jobTicker := time.NewTicker(cfg.JobPollInterval)
	defer jobTicker.Stop()
	fullCheckTicker := time.NewTicker(cfg.CheckInterval)
	defer fullCheckTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-jobTicker.C:
			if err = jobRunner.RunPending(ctx, cfg.Workers*2, cfg.Workers); err != nil {
				logger.Error("probe jobs run failed", slog.String("error", err.Error()))
				os.Exit(1)
			}
		case <-fullCheckTicker.C:
			if err = service.RunOnce(ctx); err != nil {
				logger.Error("checker run failed", slog.String("error", err.Error()))
				os.Exit(1)
			}
		}
	}
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
