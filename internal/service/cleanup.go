package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/domain"
)

// CleanupService periodically removes expired tokens from the database.
type CleanupService struct {
	tokenRepo   domain.TokenRepository
	logger      *slog.Logger
	interval    time.Duration
	retention   time.Duration // how long after expiration to actually delete
	stopCh      chan struct{}
	stoppedCh   chan struct{}
}

// NewCleanupService constructs a cleanup service with default 24h interval.
func NewCleanupService(tokenRepo domain.TokenRepository, logger *slog.Logger) *CleanupService {
	return &CleanupService{
		tokenRepo:   tokenRepo,
		logger:      logger,
		interval:    24 * time.Hour,  // run once per day
		retention:   24 * time.Hour,  // delete tokens expired > 24h ago (safety buffer)
		stopCh:      make(chan struct{}),
		stoppedCh:   make(chan struct{}),
	}
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
	// Run initial cleanup immediately
	if err := s.runCleanup(ctx); err != nil {
		s.logger.Error("initial token cleanup failed", slog.String("error", err.Error()))
		// Don't fail startup on cleanup error, just log it
	}

	go s.loop(ctx)
	s.logger.Info("token cleanup service started", slog.Duration("interval", s.interval), slog.Duration("retention", s.retention))
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
	return nil
}
