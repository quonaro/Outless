package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/domain"
)

// CleanupService periodically removes expired tokens and old domain usage from the database.
type CleanupService struct {
	tokenRepo       domain.TokenRepository
	trafficRepo     domain.TrafficRepository
	logger          *slog.Logger
	interval        time.Duration
	retention       time.Duration
	domainRetention time.Duration
	stopCh          chan struct{}
	stoppedCh       chan struct{}
}

// NewCleanupService constructs a cleanup service with default 24h interval.
func NewCleanupService(tokenRepo domain.TokenRepository, logger *slog.Logger) *CleanupService {
	return &CleanupService{
		tokenRepo:       tokenRepo,
		logger:          logger,
		interval:        24 * time.Hour,
		retention:       24 * time.Hour,
		domainRetention: 30 * 24 * time.Hour,
		stopCh:          make(chan struct{}),
		stoppedCh:       make(chan struct{}),
	}
}

// WithTrafficRepo injects the traffic repository for domain usage cleanup.
func (s *CleanupService) WithTrafficRepo(repo domain.TrafficRepository) *CleanupService {
	s.trafficRepo = repo
	return s
}

// WithDomainRetention sets how long domain usage day-records are kept.
func (s *CleanupService) WithDomainRetention(d time.Duration) *CleanupService {
	s.domainRetention = d
	return s
}

// WithInterval sets custom cleanup interval (useful for testing).
func (s *CleanupService) WithInterval(d time.Duration) *CleanupService {
	s.interval = d
	return s
}

// WithRetention sets how long after expiration tokens are actually deleted.
func (s *CleanupService) WithRetention(d time.Duration) *CleanupService {
	s.retention = d
	return s
}

// Start begins the periodic cleanup goroutine.
func (s *CleanupService) Start(ctx context.Context) error {
	if err := s.runCleanup(ctx); err != nil {
		s.logger.Error("initial token cleanup failed", slog.String("error", err.Error()))
	}
	go s.loop(ctx)
	s.logger.Info("token cleanup service started",
		slog.Duration("interval", s.interval),
		slog.Duration("retention", s.retention),
		slog.Duration("domain_retention", s.domainRetention),
	)
	return nil
}

// Stop signals the cleanup loop to stop and waits for it to finish.
func (s *CleanupService) Stop() error {
	close(s.stopCh)
	<-s.stoppedCh
	return nil
}

func (s *CleanupService) loop(ctx context.Context) {
	defer close(s.stoppedCh)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("token cleanup service stopping (context done)")
			return
		case <-s.stopCh:
			s.logger.Info("token cleanup service stopped")
			return
		case <-ticker.C:
			if err := s.runCleanup(ctx); err != nil {
				s.logger.Error("periodic token cleanup failed", slog.String("error", err.Error()))
			}
		}
	}
}

func (s *CleanupService) runCleanup(ctx context.Context) error {
	cutoff := time.Now().UTC().Add(-s.retention)
	deleted, err := s.tokenRepo.CleanupExpired(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("cleanup expired tokens: %w", err)
	}
	if deleted > 0 {
		s.logger.Info("token cleanup completed", slog.Int64("deleted", deleted))
	} else {
		s.logger.Debug("token cleanup completed, no expired tokens found")
	}

	if s.trafficRepo != nil {
		domainCutoff := time.Now().UTC().Add(-s.domainRetention)
		if err := s.trafficRepo.DeleteDomainUsageOlderThan(ctx, domainCutoff); err != nil {
			s.logger.Error("domain usage cleanup failed", slog.String("error", err.Error()))
		} else {
			s.logger.Debug("domain usage cleanup completed")
		}
	}

	return nil
}
