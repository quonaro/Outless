package checker

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"

	"outless/internal/domain"

	"golang.org/x/sync/errgroup"
)

// Config controls checker worker pool behavior.
type Config struct {
	Workers int
}

// Service checks proxy nodes concurrently and stores probe results.
type Service struct {
	repo   domain.NodeRepository
	engine domain.ProxyEngine
	logger *slog.Logger
	cfg    Config
}

// NewService builds a checker service with constructor injection.
func NewService(repo domain.NodeRepository, engine domain.ProxyEngine, logger *slog.Logger, cfg Config) *Service {
	workers := cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	return &Service{
		repo:   repo,
		engine: engine,
		logger: logger,
		cfg:    Config{Workers: workers},
	}
}

// RunOnce executes one full probe cycle over all nodes.
func (s *Service) RunOnce(ctx context.Context) error {
	jobs := make(chan domain.Node, s.cfg.Workers*2)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		defer close(jobs)
		for node, err := range s.repo.IterateNodes(groupCtx) {
			if err != nil {
				return fmt.Errorf("iterating nodes: %w", err)
			}

			select {
			case <-groupCtx.Done():
				return groupCtx.Err()
			case jobs <- node:
			}
		}
		return nil
	})

	for workerID := range s.cfg.Workers {
		workerID := workerID
		group.Go(func() error {
			for {
				select {
				case <-groupCtx.Done():
					return groupCtx.Err()
				case node, ok := <-jobs:
					if !ok {
						return nil
					}
					if err := s.checkNode(groupCtx, node); err != nil {
						return fmt.Errorf("worker %d checking node %s: %w", workerID, node.ID, err)
					}
				}
			}
		})
	}

	if err := group.Wait(); err != nil {
		return fmt.Errorf("running checker cycle: %w", err)
	}

	return nil
}

func (s *Service) checkNode(ctx context.Context, node domain.Node) error {
	result, err := s.engine.ProbeNode(ctx, node)
	if err != nil {
		s.logger.Warn("node probe failed", slog.String("node_id", node.ID), slog.String("error", err.Error()))
		result = domain.ProbeResult{
			NodeID:  node.ID,
			Status:  domain.NodeStatusUnhealthy,
			Country: node.Country,
		}
	}

	if result.NodeID == "" {
		result.NodeID = node.ID
	}

	if err = s.repo.UpdateProbeResult(ctx, result); err != nil {
		return fmt.Errorf("saving probe result for node %s: %w", node.ID, err)
	}

	s.logger.Debug("node checked", slog.String("node_id", node.ID), slog.String("status", string(result.Status)))
	return nil
}

// RunLoop starts periodic checks until context cancellation.
func (s *Service) RunLoop(ctx context.Context, ticks <-chan struct{}) error {
	var once sync.Once
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticks:
			err := s.RunOnce(ctx)
			if err != nil {
				once.Do(func() {
					s.logger.Error("checker cycle failed", slog.String("error", err.Error()))
				})
			}
		}
	}
}
