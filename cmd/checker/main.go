package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
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
	engine := xray.NewEngine(&http.Client{Timeout: 10 * time.Second}, logger, cfg.ProbeURL, cfg.XrayAdminURL)
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
	DatabaseURL   string
	Workers       int
	ProbeURL      string
	XrayAdminURL  string
	CheckInterval time.Duration
}

func loadConfig(path string, logger *slog.Logger) (Config, error) {
	loader := config.NewLoader(logger)
	yamlCfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(path, &yamlCfg); err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}

	return Config{
		DatabaseURL:   yamlCfg.Database.URL,
		Workers:       yamlCfg.Checker.Workers,
		ProbeURL:      yamlCfg.Checker.Xray.ProbeURL,
		XrayAdminURL:  yamlCfg.Checker.Xray.AdminURL,
		CheckInterval: yamlCfg.Checker.CheckInterval,
	}, nil
}

func logXrayStartupStatus(adminURL string, logger *slog.Logger) {
	checkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := checkXrayAdminConnectivity(checkCtx, adminURL); err != nil {
		logger.Error("Xray is dead", slog.String("admin_url", adminURL), slog.String("error", err.Error()))
		return
	}
	logger.Info("Xray is ready", slog.String("admin_url", adminURL))
}

func checkXrayAdminConnectivity(ctx context.Context, adminURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(adminURL))
	if err != nil {
		return fmt.Errorf("invalid xray admin url: %w", err)
	}
	if parsed.Hostname() == "" {
		return errors.New("xray admin url has empty hostname")
	}

	port := parsed.Port()
	switch {
	case port != "":
	case strings.EqualFold(parsed.Scheme, "https"):
		port = "443"
	default:
		port = "80"
	}

	address := net.JoinHostPort(parsed.Hostname(), port)
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dialing %s: %w", address, err)
	}
	_ = conn.Close()
	return nil
}
