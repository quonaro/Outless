package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"outless/internal/adapters/postgres"
	"outless/internal/adapters/xray"
	"outless/internal/app/hub"
	"outless/pkg/config"

	"golang.org/x/sync/errgroup"
)

// Config bundles runtime settings the hub process needs.
type Config struct {
	DatabaseURL string
	Hub         config.HubConfig
}

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

	db, err := postgres.NewGormDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	nodeRepo := postgres.NewGormNodeRepository(db, logger)
	tokenRepo := postgres.NewGormTokenRepository(db, logger)

	manager := hub.NewManager(tokenRepo, nodeRepo, hub.ManagerConfig{
		ConfigPath:   cfg.Hub.ConfigPath,
		XrayBinary:   cfg.Hub.XrayBinary,
		SyncInterval: cfg.Hub.SyncInterval,
		Inbound: xray.HubInboundConfig{
			Listen:      listenHost(cfg.Hub.ListenAddress),
			Port:        cfg.Hub.Port,
			SNI:         cfg.Hub.SNI,
			PrivateKey:  cfg.Hub.PrivateKey,
			ShortID:     cfg.Hub.ShortID,
			Destination: cfg.Hub.SNI + ":443",
		},
	}, logger)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return manager.Run(groupCtx)
	})

	logger.Info("hub started",
		slog.String("config_path", cfg.Hub.ConfigPath),
		slog.String("xray_binary", cfg.Hub.XrayBinary),
		slog.Duration("sync_interval", cfg.Hub.SyncInterval),
	)

	if err := group.Wait(); err != nil && ctx.Err() == nil {
		logger.Error("hub exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("hub shut down cleanly")
}

func loadConfig(path string, logger *slog.Logger) (Config, error) {
	loader := config.NewLoader(logger)
	yamlCfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(path, &yamlCfg); err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}

	return Config{
		DatabaseURL: yamlCfg.Database.URL,
		Hub:         yamlCfg.Hub,
	}, nil
}

// listenHost strips the port from a ":443"-style listen address.
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
