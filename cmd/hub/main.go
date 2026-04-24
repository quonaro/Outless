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

	"outless/internal/adapters/postgres"
	"outless/internal/adapters/xray"
	"outless/internal/app/hub"
	"outless/pkg/config"
	"outless/pkg/logging"

	"golang.org/x/sync/errgroup"
)

// Config bundles runtime settings the hub process needs.
type Config struct {
	DatabaseURL string
	Hub         HubConfig
}

// HubConfig bundles validated runtime and inbound settings for hub process.
type HubConfig struct {
	config.HubConfig
	RuntimeMode config.XrayRuntimeMode
}

func main() {
	configPath := flag.String("config", "outless.yaml", "path to config file")
	flag.Parse()

	logger := logging.New("hub")
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

	runtimeController, err := buildRuntimeController(cfg.Hub, logger)
	if err != nil {
		logger.Error("invalid xray runtime config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	manager := hub.NewManager(tokenRepo, nodeRepo, runtimeController, hub.ManagerConfig{
		ConfigPath:   cfg.Hub.ConfigPath,
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
		slog.String("runtime_mode", string(cfg.Hub.RuntimeMode)),
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
		Hub: HubConfig{
			HubConfig: config.HubConfig{
				Host:          yamlCfg.Hub.Host,
				Port:          yamlCfg.Hub.Port,
				SNI:           yamlCfg.Hub.SNI,
				PublicKey:     yamlCfg.Hub.PublicKey,
				PrivateKey:    yamlCfg.Hub.PrivateKey,
				ShortID:       yamlCfg.Hub.ShortID,
				Fingerprint:   yamlCfg.Hub.Fingerprint,
				ListenAddress: yamlCfg.Hub.ListenAddress,
				ConfigPath:    yamlCfg.Xray.Edge.ConfigPath,
				XrayBinary:    yamlCfg.Xray.Edge.XrayBinary,
				SyncInterval:  yamlCfg.Hub.SyncInterval,
			},
			RuntimeMode: yamlCfg.Xray.Edge.RuntimeMode,
		},
	}, nil
}

func buildRuntimeController(cfg HubConfig, logger *slog.Logger) (hub.RuntimeController, error) {
	if strings.TrimSpace(cfg.PrivateKey) == "" {
		return nil, errors.New("hub.private_key is required")
	}
	if strings.TrimSpace(cfg.PublicKey) == "" {
		return nil, errors.New("hub.public_key is required")
	}
	if strings.TrimSpace(cfg.ConfigPath) == "" {
		return nil, errors.New("xray.edge.config_path is required")
	}

	switch cfg.RuntimeMode {
	case config.XrayRuntimeEmbedded:
		if strings.TrimSpace(cfg.XrayBinary) == "" {
			return nil, errors.New("xray.edge.xray_binary is required in embedded mode")
		}
		return xray.NewEmbeddedRuntimeController(logger, cfg.XrayBinary), nil
	case config.XrayRuntimeExternal:
		if strings.TrimSpace(cfg.XrayBinary) != "" {
			logger.Warn("xray.edge.xray_binary is ignored in external mode", slog.String("xray_binary", cfg.XrayBinary))
		}
		// Use DockerRuntimeController to send HUP signal via docker CLI
		// This requires docker.sock to be mounted in the hub container
		return xray.NewDockerRuntimeController(logger, "outless-xray-edge"), nil
	default:
		return nil, fmt.Errorf("unsupported xray.edge.runtime_mode: %q", cfg.RuntimeMode)
	}
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
