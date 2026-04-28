package service

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

	"outless/internal/adapter/xray"
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
	// RemoveUser removes a client from the inbound by email
	RemoveUser(email string) error
	// RemoveRulesForUser removes all routing rules for a specific user email
	RemoveRulesForUser(email string) error
	// ForceSync immediately syncs Xray config with current DB state
	ForceSync() error
}

// RouterManager keeps edge Xray config in sync with DB and delegates runtime lifecycle.
type RouterManager struct {
	tokenRepo domain.TokenRepository
	nodeRepo  domain.NodeRepository
	runtime   RuntimeController
	cfg       ManagerConfig
	logger    *slog.Logger

	mu               sync.Mutex
	lastSum          string
	lastActiveTokens int
	runtimeActive    bool
}

// NewRouterManager builds a Hub manager.
func NewRouterManager(tokenRepo domain.TokenRepository, nodeRepo domain.NodeRepository, runtime RuntimeController, cfg ManagerConfig, logger *slog.Logger) *RouterManager {
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = 30 * time.Second
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = "/var/lib/outless/xray-hub.json"
	}
	if runtime == nil {
		runtime = noopRuntimeController{}
	}

	return &RouterManager{
		tokenRepo:        tokenRepo,
		nodeRepo:         nodeRepo,
		runtime:          runtime,
		cfg:              cfg,
		logger:           logger,
		lastActiveTokens: -1, // Initialize to -1 to force first reload
	}
}

// Run executes initial sync, connects to external Xray via gRPC API and keeps
// the config refreshed until ctx is cancelled. It blocks until context terminates.
func (m *RouterManager) Run(ctx context.Context) error {
	if err := m.Sync(ctx); err != nil {
		return fmt.Errorf("initial hub sync: %w", err)
	}

	if err := m.runtime.Start(m.cfg.ConfigPath); err != nil {
		return fmt.Errorf("starting runtime (%s): %w", m.runtime.Description(), err)
	}
	m.runtimeActive = true
	m.logger.Info("xray runtime started", slog.String("controller", m.runtime.Description()))

	if err := m.runtime.Reload(m.cfg.ConfigPath); err != nil {
		return fmt.Errorf("initial runtime reload (%s): %w", m.runtime.Description(), err)
	}
	m.logger.Info("xray runtime initial reload completed", slog.String("controller", m.runtime.Description()))

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
func (m *RouterManager) Sync(ctx context.Context) error {
	now := time.Now().UTC()

	tokens, err := m.tokenRepo.ListActive(ctx, now)
	if err != nil {
		return fmt.Errorf("listing active tokens: %w", err)
	}

	nodes, err := m.nodeRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("listing nodes: %w", err)
	}

	m.logger.Debug("Sync: loaded tokens and nodes",
		slog.Int("tokens", len(tokens)),
		slog.Int("nodes", len(nodes)),
	)

	// Log token details for debugging
	for _, token := range tokens {
		m.logger.Debug("Active token",
			slog.String("id", token.ID),
			slog.String("uuid", token.UUID),
			slog.String("group", token.GroupID),
			slog.Any("groups", token.GroupIDs),
		)
	}

	// Log node details for debugging
	for _, node := range nodes {
		m.logger.Debug("Node",
			slog.String("id", node.ID),
			slog.String("group", node.GroupID),
			slog.String("country", node.Country),
		)
	}

	m.logger.Debug("Generating Xray config with inbound settings",
		slog.String("inbound_dest", m.cfg.Inbound.Destination),
		slog.String("inbound_sni", m.cfg.Inbound.SNI),
		slog.String("inbound_shortid", m.cfg.Inbound.ShortID),
		slog.Bool("has_private_key", m.cfg.Inbound.PrivateKey != ""),
	)

	payload, err := xray.GenerateHubConfig(tokens, nodes, m.cfg.Inbound, m.logger)
	if err != nil {
		return fmt.Errorf("generating xray config: %w", err)
	}

	sum := checksum(payload)
	activeTokens := len(tokens)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Reload if config changed OR number of active tokens changed
	// This ensures Xray restarts even when going from 0 to 0 tokens (same empty config)
	configChanged := sum != m.lastSum
	tokensChanged := activeTokens != m.lastActiveTokens

	if !configChanged && !tokensChanged {
		return nil
	}

	if err := writeAtomic(m.cfg.ConfigPath, payload); err != nil {
		return fmt.Errorf("writing xray config: %w", err)
	}
	m.lastSum = sum
	m.lastActiveTokens = activeTokens

	m.logger.Info("xray config updated",
		slog.Int("tokens", activeTokens),
		slog.Int("nodes", len(nodes)),
		slog.String("path", m.cfg.ConfigPath),
		slog.Bool("config_changed", configChanged),
		slog.Bool("tokens_changed", tokensChanged),
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

func (noopRuntimeController) Start(string) error              { return nil }
func (noopRuntimeController) Reload(string) error             { return nil }
func (noopRuntimeController) Stop()                           {}
func (noopRuntimeController) Description() string             { return "noop" }
func (noopRuntimeController) RemoveUser(string) error         { return nil }
func (noopRuntimeController) RemoveRulesForUser(string) error { return nil }
func (noopRuntimeController) ForceSync() error                { return nil }
