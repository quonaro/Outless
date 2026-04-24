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

	"outless/internal/adapters/docker"
	"outless/pkg/config"
	"outless/pkg/logging"
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

	dockerManager, err := docker.NewContainerManager(logger)
	if err != nil {
		logger.Error("failed to create docker manager", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dockerManager.Close()

	svc := NewService(dockerManager, cfg.ShardCount, logger)

	// Initial sync
	if err := svc.Sync(ctx); err != nil {
		logger.Error("initial sync failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Periodic sync
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down probe manager")
			return
		case <-ticker.C:
			if err := svc.Sync(ctx); err != nil {
				logger.Error("sync failed", slog.String("error", err.Error()))
			}
		}
	}
}

// Config holds probe manager configuration.
type Config struct {
	ShardCount int
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
		ShardCount: yamlCfg.Xray.Probe.ShardCount,
	}

	if cfg.ShardCount <= 0 {
		return Config{}, fmt.Errorf("shard_count must be greater than 0")
	}

	return cfg, nil
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
