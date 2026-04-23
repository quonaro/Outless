package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"outless/internal/adapters/postgres"
	"outless/internal/adapters/xray"
	"outless/internal/app/checker"
	"outless/pkg/config"
)

func main() {
	configPath := flag.String("config", "outless.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
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
	engine := xray.NewEngine(logger, cfg.ProbeURL, cfg.XrayAdminURL, cfg.XraySocksAddr, xray.GeoIPConfig{
		DBPath: cfg.XrayGeoIPDBPath,
		DBURL:  cfg.XrayGeoIPDBURL,
		Auto:   cfg.XrayGeoIPAuto,
		TTL:    cfg.XrayGeoIPTTL,
	}, 10*time.Second)
	service := checker.NewService(repo, engine, logger, checker.Config{Workers: cfg.Workers})

	// Run periodic checks with ticker
	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
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
	CheckInterval   time.Duration
}

func loadConfig(path string, logger *slog.Logger) (Config, error) {
	loader := config.NewLoader(logger)
	yamlCfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(path, &yamlCfg); err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}

	return Config{
		DatabaseURL:     yamlCfg.Database.URL,
		Workers:         yamlCfg.Checker.Workers,
		ProbeURL:        yamlCfg.Checker.Xray.ProbeURL,
		XrayAdminURL:    yamlCfg.Checker.Xray.AdminURL,
		XraySocksAddr:   yamlCfg.Checker.Xray.SocksAddr,
		XrayGeoIPDBPath: yamlCfg.Checker.Xray.GeoIPDBPath,
		XrayGeoIPDBURL:  yamlCfg.Checker.Xray.GeoIPDBURL,
		XrayGeoIPAuto:   yamlCfg.Checker.Xray.GeoIPAuto,
		XrayGeoIPTTL:    yamlCfg.Checker.Xray.GeoIPTTL,
		CheckInterval:   yamlCfg.Checker.CheckInterval,
	}, nil
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
