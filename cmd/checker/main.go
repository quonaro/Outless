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
	logXrayStartupStatus(cfg.XrayAdminURL, logger)

	db, err := postgres.NewGormDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	repo := postgres.NewGormNodeRepository(db, logger)
	jobRepo := postgres.NewGormProbeJobRepository(db, logger)
	engine := xray.NewEngine(logger, cfg.ProbeURL, cfg.XrayAdminURL, cfg.XraySocksAddr, xray.GeoIPConfig{
		DBPath: cfg.XrayGeoIPDBPath,
		DBURL:  cfg.XrayGeoIPDBURL,
		Auto:   cfg.XrayGeoIPAuto,
		TTL:    cfg.XrayGeoIPTTL,
	}, 10*time.Second)
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
	if strings.TrimSpace(cfg.XrayAdminURL) == "" {
		return errors.New("xray.probe.admin_url is required")
	}
	if strings.TrimSpace(cfg.XraySocksAddr) == "" {
		return errors.New("xray.probe.socks_addr is required")
	}
	if strings.TrimSpace(cfg.ProbeURL) == "" {
		return errors.New("xray.probe.probe_url is required")
	}
	if cfg.JobPollInterval <= 0 {
		return errors.New("checker.job_poll_interval must be greater than 0")
	}
	if cfg.CheckInterval <= 0 {
		return errors.New("checker.check_interval must be greater than 0")
	}
	return nil
}

func logXrayStartupStatus(adminURL string, logger *slog.Logger) {
	checkCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := xray.CheckXrayAPI(checkCtx, adminURL); err != nil {
		logger.Error("Xray is dead", slog.String("admin_url", adminURL), slog.String("error", err.Error()))
		return
	}
	logger.Info("Xray is ready", slog.String("admin_url", adminURL))
}
