package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"outless/internal/domain"
	"outless/internal/topup/checker"
)

// TopUpRunResult holds the outcome of a single top-up run.
type TopUpRunResult struct {
	TopUpID string
	GroupID string
	Total   int
	Checked int
	Passed  int
	Added   int
	Failed  bool
	Error   string
}

// TopUpScheduler periodically fetches, validates, and imports nodes for top-up groups.
type TopUpScheduler struct {
	topUpRepo domain.GroupTopUpRepository
	groupRepo domain.GroupRepository
	nodeRepo  domain.NodeRepository
	checker   *checker.Checker
	logger    *slog.Logger
	interval  time.Duration
	progress  *progressBroadcaster

	stop chan struct{}
	wg   sync.WaitGroup
	mu   sync.Map // top-up id -> *sync.Mutex
}

// NewTopUpScheduler creates a new top-up scheduler.
func NewTopUpScheduler(
	topUpRepo domain.GroupTopUpRepository,
	groupRepo domain.GroupRepository,
	nodeRepo domain.NodeRepository,
	check *checker.Checker,
	logger *slog.Logger,
) *TopUpScheduler {
	return &TopUpScheduler{
		topUpRepo: topUpRepo,
		groupRepo: groupRepo,
		nodeRepo:  nodeRepo,
		checker:   check,
		logger:    logger,
		interval:  time.Minute,
		progress:  newProgressBroadcaster(),
		stop:      make(chan struct{}),
	}
}

// Start begins the scheduler loop.
func (s *TopUpScheduler) Start() {
	s.wg.Add(1)
	go s.loop()
}

// Stop gracefully stops the scheduler loop.
func (s *TopUpScheduler) Stop() {
	close(s.stop)
	s.wg.Wait()
}

// runContext returns a context that is canceled when the scheduler is stopped
// or when the parent context is canceled. Callers must call the returned cancel.
func (s *TopUpScheduler) runContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func (s *TopUpScheduler) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			ctx, cancel := s.runContext(context.Background())
			s.runTick(ctx)
			cancel()
		}
	}
}

func (s *TopUpScheduler) runTick(ctx context.Context) {
	s.logger.Debug("top-up tick started")
	topUps, err := s.topUpRepo.ListDue(ctx, time.Now().UTC())
	if err != nil {
		s.logger.Error("failed to list due top-ups", slog.String("error", err.Error()))
		return
	}

	s.logger.Debug("due top-ups listed", slog.Int("count", len(topUps)))

	for _, topUp := range topUps {
		topUp := topUp
		mu := s.getOrCreateLock(topUp.ID)
		if !mu.TryLock() {
			s.logger.Debug("top-up run already in progress, skipping", slog.String("id", topUp.ID))
			continue
		}
		s.logger.Debug("starting scheduled top-up run", slog.String("id", topUp.ID))

		if err := s.updateNextRunBeforeJob(ctx, &topUp); err != nil {
			s.logger.Error("failed to update next run for top-up", slog.String("id", topUp.ID), slog.String("error", err.Error()))
			mu.Unlock()
			continue
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer mu.Unlock()
			runCtx, cancel := s.runContext(context.Background())
			defer cancel()
			_, _ = s.runTopUp(runCtx, topUp)
		}()
	}
}

// RunAsync triggers a top-up run in the background and ties it to the scheduler lifecycle.
func (s *TopUpScheduler) RunAsync(id string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := s.runContext(context.Background())
		defer cancel()
		if _, err := s.RunNow(ctx, id); err != nil {
			s.logger.Error("failed to run top-up", slog.String("id", id), slog.String("error", err.Error()))
		}
	}()
}

// RunAllAsync triggers all enabled top-up runs in the background and ties them to the scheduler lifecycle.
func (s *TopUpScheduler) RunAllAsync() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := s.runContext(context.Background())
		defer cancel()
		runResults := s.RunAll(ctx)
		s.logger.Debug("background run all top-ups completed", slog.Int("count", len(runResults)))
	}()
}

func (s *TopUpScheduler) getOrCreateLock(id string) *sync.Mutex {
	mu, _ := s.mu.LoadOrStore(id, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// RunNow triggers a top-up run immediately and returns the run result.
func (s *TopUpScheduler) RunNow(ctx context.Context, id string) (TopUpRunResult, error) {
	var result TopUpRunResult
	s.logger.Debug("run top-up requested", slog.String("id", id))
	topUp, err := s.topUpRepo.FindByID(ctx, id)
	if err != nil {
		return result, err
	}
	s.logger.Debug("top-up loaded, acquiring lock", slog.String("id", id), slog.String("group_id", topUp.GroupID))

	mu := s.getOrCreateLock(id)
	mu.Lock()
	s.logger.Debug("top-up lock acquired", slog.String("id", id))
	defer mu.Unlock()

	return s.runTopUp(ctx, topUp)
}

func (s *TopUpScheduler) updateNextRunBeforeJob(ctx context.Context, topUp *domain.GroupTopUp) error {
	next, keepEnabled, err := computeNextRun(*topUp, time.Now().UTC())
	if err != nil {
		return err
	}
	topUp.NextRunAt = next
	topUp.Enabled = keepEnabled
	if err := s.topUpRepo.Update(ctx, *topUp); err != nil {
		return err
	}
	return nil
}

// RunAll runs every enabled top-up and returns a result for each.
func (s *TopUpScheduler) RunAll(ctx context.Context) []TopUpRunResult {
	s.logger.Debug("run all top-ups started")
	topUps, err := s.topUpRepo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list top-ups", slog.String("error", err.Error()))
		return nil
	}

	results := make([]TopUpRunResult, 0, len(topUps))
	var mu sync.Mutex
	var wg sync.WaitGroup

	s.logger.Debug("top-ups listed", slog.Int("count", len(topUps)))

	for _, topUp := range topUps {
		if !topUp.Enabled {
			s.logger.Debug("skipping disabled top-up", slog.String("id", topUp.ID))
			continue
		}
		topUp := topUp
		s.logger.Debug("queuing top-up run", slog.String("id", topUp.ID))
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.RunNow(ctx, topUp.ID)
			if err != nil {
				r.Failed = true
				r.Error = err.Error()
			}
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}()
	}
	wg.Wait()

	s.logger.Debug("run all top-ups finished", slog.Int("results", len(results)))
	return results
}
