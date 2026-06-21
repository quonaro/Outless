package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/domain"
)

// RouterManager owns the embedded runtime lifecycle and keeps it in sync with
// the database by periodically requesting a (debounced) reload.
type RouterManager struct {
	runtime  domain.RuntimeController
	interval time.Duration
	logger   *slog.Logger
}

// NewRouterManager builds a router manager around a runtime controller.
func NewRouterManager(runtime domain.RuntimeController, interval time.Duration, logger *slog.Logger) *RouterManager {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &RouterManager{runtime: runtime, interval: interval, logger: logger}
}

// Run starts the runtime and keeps it synced until ctx is canceled.
func (m *RouterManager) Run(ctx context.Context) error {
	if err := m.runtime.Start(ctx); err != nil {
		return fmt.Errorf("starting runtime (%s): %w", m.runtime.Description(), err)
	}
	m.logger.Info("runtime started", slog.String("controller", m.runtime.Description()))

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("router manager shutting down")
			m.runtime.Stop()
			return nil
		case <-ticker.C:
			if err := m.runtime.Reload(); err != nil {
				m.logger.Warn("runtime reload failed", slog.String("error", err.Error()))
			}
		}
	}
}
