package hub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"outless/internal/adapters/xray"
	"outless/internal/domain"
)

// ManagerConfig holds runtime settings for the Hub manager.
type ManagerConfig struct {
	ConfigPath   string
	XrayBinary   string
	SyncInterval time.Duration
	Inbound      xray.HubInboundConfig
}

// Manager owns the Xray child process and keeps its config in sync with the DB.
type Manager struct {
	tokenRepo domain.TokenRepository
	nodeRepo  domain.NodeRepository
	cfg       ManagerConfig
	logger    *slog.Logger

	mu      sync.Mutex
	lastSum string
	cmd     *exec.Cmd
}

// NewManager builds a Hub manager.
func NewManager(tokenRepo domain.TokenRepository, nodeRepo domain.NodeRepository, cfg ManagerConfig, logger *slog.Logger) *Manager {
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = 30 * time.Second
	}
	if cfg.XrayBinary == "" {
		cfg.XrayBinary = "xray"
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = "/var/lib/outless/xray-hub.json"
	}

	return &Manager{
		tokenRepo: tokenRepo,
		nodeRepo:  nodeRepo,
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

	if err := m.startXray(); err != nil {
		return fmt.Errorf("starting xray: %w", err)
	}

	ticker := time.NewTicker(m.cfg.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("hub manager shutting down")
			m.stopXray()
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

	if m.cmd != nil {
		if err := m.reloadXrayLocked(); err != nil {
			m.logger.Warn("xray reload failed, restart pending", slog.String("error", err.Error()))
		}
	}

	return nil
}

func (m *Manager) startXray() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil {
		return errors.New("xray already running")
	}

	cmd := exec.Command(m.cfg.XrayBinary, "run", "-c", m.cfg.ConfigPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting xray process: %w", err)
	}

	m.cmd = cmd
	m.logger.Info("xray started", slog.Int("pid", cmd.Process.Pid), slog.String("binary", m.cfg.XrayBinary))
	return nil
}

func (m *Manager) stopXray() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd == nil || m.cmd.Process == nil {
		return
	}
	if err := m.cmd.Process.Signal(os.Interrupt); err != nil {
		m.logger.Warn("xray interrupt failed", slog.String("error", err.Error()))
		_ = m.cmd.Process.Kill()
	}
	_ = m.cmd.Wait()
	m.cmd = nil
}

// reloadXrayLocked restarts the child process to pick up the new config.
// Xray has no universal SIGHUP reload across versions, so we do a clean restart.
// Caller must hold m.mu.
func (m *Manager) reloadXrayLocked() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	if err := m.cmd.Process.Signal(os.Interrupt); err != nil {
		_ = m.cmd.Process.Kill()
	}
	_ = m.cmd.Wait()
	m.cmd = nil

	cmd := exec.Command(m.cfg.XrayBinary, "run", "-c", m.cfg.ConfigPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("restarting xray: %w", err)
	}
	m.cmd = cmd
	m.logger.Info("xray restarted", slog.Int("pid", cmd.Process.Pid))
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
