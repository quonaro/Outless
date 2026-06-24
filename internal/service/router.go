package service

import (
	"context"
	"fmt"
	"log/slog"

	"outless/internal/domain"
)

// RouterManager owns the embedded runtime lifecycle and starts/stops it. Reloads
// are now event-driven (triggered by handlers when config actually changes).
type RouterManager struct {
	runtime domain.RuntimeController
	logger  *slog.Logger
}

// NewRouterManager builds a router manager around a runtime controller.
func NewRouterManager(runtime domain.RuntimeController, logger *slog.Logger) *RouterManager {
	return &RouterManager{runtime: runtime, logger: logger}
}

// Run starts the runtime and waits until ctx is canceled.
func (m *RouterManager) Run(ctx context.Context) error {
	if err := m.runtime.Start(ctx); err != nil {
		return fmt.Errorf("starting runtime (%s): %w", m.runtime.Description(), err)
	}
	m.logger.Info("runtime started", slog.String("controller", m.runtime.Description()))

	<-ctx.Done()
	m.logger.Info("router manager shutting down")
	m.runtime.Stop()
	return nil
}
