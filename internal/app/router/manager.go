package router

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"outless/internal/adapters/xray"
	"outless/internal/domain"
)

// ManagerConfig holds runtime settings for the Hub manager.
type ManagerConfig struct {
	ConfigPath   string
	SyncInterval time.Duration
	Inbound      xray.HubInboundConfig
}

// RuntimeController abstracts how hub starts/reloads/stops edge Xray runtime.
type RuntimeController interface {
	Start(configPath string) error
	Reload(configPath string) error
	Stop()
	Description() string
}

// Manager keeps edge Xray config in sync with DB and delegates runtime lifecycle.
type Manager struct {
	tokenRepo domain.TokenRepository
	nodeRepo  domain.NodeRepository
	runtime   RuntimeController
	cfg       ManagerConfig
	logger    *slog.Logger

	mu            sync.Mutex
	lastSum       string
	runtimeActive bool
}

// NewManager builds a Hub manager.
func NewManager(tokenRepo domain.TokenRepository, nodeRepo domain.NodeRepository, runtime RuntimeController, cfg ManagerConfig, logger *slog.Logger) *Manager {
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = 30 * time.Second
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = "/var/lib/outless/xray-hub.json"
	}
	if runtime == nil {
		runtime = noopRuntimeController{}
	}

	return &Manager{
		tokenRepo: tokenRepo,
		nodeRepo:  nodeRepo,
		runtime:   runtime,
		cfg:       cfg,
		logger:    logger,
	}
}

// Run executes initial sync, starts the Xray process and keeps the config
// refreshed until ctx is cancelled. It blocks until either Xray exits with an
// error or the context terminates.
func (m *Manager) Run(ctx context.Context) error {
	if err := m.Sync(ctx); err != nil {
		return fmt.Errorf("initial hub sync: %w", err)
	}

	if err := m.runtime.Start(m.cfg.ConfigPath); err != nil {
		return fmt.Errorf("starting runtime (%s): %w", m.runtime.Description(), err)
	}
	m.runtimeActive = true
	m.logger.Info("xray runtime started", slog.String("controller", m.runtime.Description()))

	ticker := time.NewTicker(m.cfg.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("hub manager shutting down")
			m.runtime.Stop()
			return nil
		case <-ticker.C:
			if err := m.Sync(ctx); err != nil {
				m.logger.Warn("hub sync failed", slog.String("error", err.Error()))
			}
		}
	}
}

// Sync regenerates Xray config from DB state and reloads Xray if the config
// content actually changed.
func (m *Manager) Sync(ctx context.Context) error {
	now := time.Now().UTC()

	tokens, err := m.tokenRepo.ListActive(ctx, now)
	if err != nil {
		return fmt.Errorf("listing active tokens: %w", err)
	}

	nodes, err := m.nodeRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("listing nodes: %w", err)
	}

	payload, err := xray.GenerateHubConfig(tokens, nodes, m.cfg.Inbound)
	if err != nil {
		return fmt.Errorf("generating xray config: %w", err)
	}

	sum := checksum(payload)

	m.mu.Lock()
	defer m.mu.Unlock()

	if sum == m.lastSum {
		return nil
	}

	if err := writeAtomic(m.cfg.ConfigPath, payload); err != nil {
		return fmt.Errorf("writing xray config: %w", err)
	}
	m.lastSum = sum
	m.logger.Info("xray config updated",
		slog.Int("tokens", len(tokens)),
		slog.Int("nodes", len(nodes)),
		slog.String("path", m.cfg.ConfigPath),
	)

	if m.runtimeActive {
		if err := m.runtime.Reload(m.cfg.ConfigPath); err != nil {
			m.logger.Warn("xray reload failed, restart pending", slog.String("error", err.Error()))
		}
	}

	return nil
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensuring config dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp config: %w", err)
	}
	return nil
}

type noopRuntimeController struct{}

func (noopRuntimeController) Start(string) error  { return nil }
func (noopRuntimeController) Reload(string) error { return nil }
func (noopRuntimeController) Stop()               {}
func (noopRuntimeController) Description() string { return "noop" }
