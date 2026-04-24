package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"outless/internal/app/nodeprobe"
	"outless/internal/domain"

	"golang.org/x/sync/errgroup"
)

// JobRunner consumes async probe jobs and executes them through ProbeNode engine.
type JobRunner struct {
	jobs   domain.ProbeJobRepository
	nodes  domain.NodeRepository
	engine domain.ProxyEngine
	logger *slog.Logger
}

// NewJobRunner constructs an async probe job executor.
func NewJobRunner(jobs domain.ProbeJobRepository, nodes domain.NodeRepository, engine domain.ProxyEngine, logger *slog.Logger) *JobRunner {
	return &JobRunner{
		jobs:   jobs,
		nodes:  nodes,
		engine: engine,
		logger: logger,
	}
}

// RunPending claims up to limit pending jobs and processes them with worker pool.
func (r *JobRunner) RunPending(ctx context.Context, limit int, workers int) error {
	if limit <= 0 {
		limit = 100
	}
	if workers <= 0 {
		workers = 1
	}

	claimed, err := r.jobs.ClaimPending(ctx, limit)
	if err != nil {
		return fmt.Errorf("claim pending probe jobs: %w", err)
	}
	if len(claimed) == 0 {
		return nil
	}

	jobsCh := make(chan domain.ProbeJob, len(claimed))
	for _, job := range claimed {
		jobsCh <- job
	}
	close(jobsCh)

	group, groupCtx := errgroup.WithContext(ctx)
	for i := range workers {
		workerID := i
		group.Go(func() error {
			for {
				select {
				case <-groupCtx.Done():
					return groupCtx.Err()
				case job, ok := <-jobsCh:
					if !ok {
						return nil
					}
					if err := r.processJob(groupCtx, job); err != nil {
						if r.logger != nil {
							r.logger.Warn("probe job processing failed",
								slog.String("job_id", job.ID),
								slog.Int("worker_id", workerID),
								slog.String("error", err.Error()),
							)
						}
					}
				}
			}
		})
	}

	if err := group.Wait(); err != nil && !strings.Contains(err.Error(), context.Canceled.Error()) {
		return fmt.Errorf("running probe jobs: %w", err)
	}
	return nil
}

func (r *JobRunner) processJob(ctx context.Context, job domain.ProbeJob) error {
	node, err := r.nodes.FindByID(ctx, job.NodeID)
	if err != nil {
		_ = r.jobs.MarkFailed(ctx, job.ID, fmt.Sprintf("loading node: %v", err))
		return fmt.Errorf("finding node %s: %w", job.NodeID, err)
	}

	probeCtx := ctx
	probeCtx = domain.WithProbeMode(probeCtx, job.Mode)
	probeCtx = domain.WithProbeURL(probeCtx, job.ProbeURL)
	result := nodeprobe.ProbeWithEngine(probeCtx, r.engine, node)

	if err := r.nodes.UpdateProbeResult(ctx, result); err != nil {
		_ = r.jobs.MarkFailed(ctx, job.ID, fmt.Sprintf("saving result: %v", err))
		return fmt.Errorf("saving probe result for node %s: %w", node.ID, err)
	}
	if err := r.jobs.MarkSucceeded(ctx, job.ID); err != nil {
		return fmt.Errorf("marking job %s succeeded: %w", job.ID, err)
	}
	return nil
}
