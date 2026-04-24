package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"outless/internal/adapters/docker"
	"outless/pkg/config"
	"outless/pkg/logging"

	"golang.org/x/sync/errgroup"
)

func main() {
	configPath := flag.String("config", "outless.yaml", "path to config file")
	flag.Parse()

	logger := logging.New("probe-manager")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig(*configPath, logger)
	if err != nil {
		logger.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

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

	svc := NewService(dockerManager, cfg.ShardCount, logger)

	// Initial sync
	if err := svc.Sync(ctx); err != nil {
		logger.Error("initial sync failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Periodic sync with graceful shutdown
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-gCtx.Done():
				logger.Info("probe manager shutting down gracefully")
				return gCtx.Err()
			case <-ticker.C:
				if err := svc.Sync(gCtx); err != nil {
					logger.Error("sync failed", slog.String("error", err.Error()))
				}
			}
		}
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("probe manager exited with error", slog.String("error", err.Error()))
	}

	// Cleanup on exit
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := svc.Cleanup(cleanupCtx); err != nil {
		logger.Error("cleanup failed", slog.String("error", err.Error()))
	}

	logger.Info("probe manager shutdown complete")
}

// Config holds probe manager configuration.
type Config struct {
	ShardCount     int
	XrayConfigPath string
	ConfigVolume   string
}

func loadConfig(path string, logger *slog.Logger) (Config, error) {
	loader := config.NewLoader(logger)
	yamlCfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(path, &yamlCfg); err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}

	// Apply compatibility to generate shards from shard_count
	yamlCfg.ApplyCompatibility()

	cfg := Config{
		ShardCount:     yamlCfg.Xray.Probe.ShardCount,
		XrayConfigPath: os.Getenv("XRAY_CONFIG_PATH"),
		ConfigVolume:   os.Getenv("XRAY_CONFIG_VOLUME"),
	}

	if cfg.XrayConfigPath == "" {
		cfg.XrayConfigPath = "/app/xray/config.json"
	}
	if cfg.ConfigVolume == "" {
		cfg.ConfigVolume = "xray_config"
	}

	if cfg.ShardCount <= 0 {
		return Config{}, fmt.Errorf("shard_count must be greater than 0")
	}

	return cfg, nil
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

// Service manages probe container lifecycle.
type Service struct {
	docker     *docker.ContainerManager
	shardCount int
	logger     *slog.Logger
}

// NewService creates a new probe manager service.
func NewService(dockerMgr *docker.ContainerManager, shardCount int, logger *slog.Logger) *Service {
	return &Service{
		docker:     dockerMgr,
		shardCount: shardCount,
		logger:     logger,
	}
}

// Sync ensures the correct number of probe containers exist.
func (s *Service) Sync(ctx context.Context) error {
	existing, err := s.docker.ListProbeContainers(ctx)
	if err != nil {
		return fmt.Errorf("listing existing containers: %w", err)
	}

	existingMap := make(map[string]bool)
	for _, name := range existing {
		existingMap[name] = true
	}

	// Create missing containers
	for i := 1; i <= s.shardCount; i++ {
		name := fmt.Sprintf("xray-probe-%d", i)
		if !existingMap[name] {
			if err := s.docker.CreateProbeContainer(ctx, name); err != nil {
				return fmt.Errorf("creating container %s: %w", name, err)
			}
		}
	}

	// Remove excess containers
	for _, name := range existing {
		shardNum := 0
		if _, err := fmt.Sscanf(name, "xray-probe-%d", &shardNum); err == nil {
			if shardNum > s.shardCount {
				if err := s.docker.RemoveProbeContainer(ctx, name); err != nil {
					return fmt.Errorf("removing container %s: %w", name, err)
				}
			}
		}
	}

	s.logger.Info("probe containers synced", slog.Int("desired", s.shardCount), slog.Int("existing", len(existing)))
	return nil
}

// Cleanup removes all managed probe containers.
func (s *Service) Cleanup(ctx context.Context) error {
	s.logger.Info("cleaning up probe containers")
	existing, err := s.docker.ListProbeContainers(ctx)
	if err != nil {
		return fmt.Errorf("listing containers for cleanup: %w", err)
	}

	for _, name := range existing {
		if err := s.docker.RemoveProbeContainer(ctx, name); err != nil {
			s.logger.Warn("failed to remove container during cleanup",
				slog.String("name", name),
				slog.String("error", err.Error()))
		}
	}

	return nil
}
