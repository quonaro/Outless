package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"outless/internal/country"
	"outless/internal/domain"
	"outless/shared/vless"
)

const (
	defaultWatcherInterval = 30 * time.Second
	defaultBatchSize       = 50
	defaultRetryDelay      = 10 * time.Minute
)

// WatcherConfig tunes background country lookup behavior.
type WatcherConfig struct {
	Interval   time.Duration
	BatchSize  int
	RetryDelay time.Duration
}

// DefaultWatcherConfig returns sensible defaults for the country watcher.
func DefaultWatcherConfig() WatcherConfig {
	return WatcherConfig{
		Interval:   defaultWatcherInterval,
		BatchSize:  defaultBatchSize,
		RetryDelay: defaultRetryDelay,
	}
}

// CountryWatcher periodically resolves missing country information for nodes.
type CountryWatcher struct {
	nodeRepo domain.NodeRepository
	resolver *country.Resolver
	logger   *slog.Logger
	cfg      WatcherConfig
}

// NewCountryWatcher creates a new background country lookup watcher.
func NewCountryWatcher(
	nodeRepo domain.NodeRepository,
	resolver *country.Resolver,
	logger *slog.Logger,
	cfg WatcherConfig,
) *CountryWatcher {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultWatcherInterval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = defaultRetryDelay
	}
	if resolver == nil {
		resolver = country.NewResolver(&http.Client{Timeout: 10 * time.Second})
	}
	return &CountryWatcher{
		nodeRepo: nodeRepo,
		resolver: resolver,
		logger:   logger,
		cfg:      cfg,
	}
}

// Run starts the watcher and blocks until the context is canceled.
func (w *CountryWatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	w.processBatch(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

// RunAsync starts the watcher in the background.
func (w *CountryWatcher) RunAsync(ctx context.Context) {
	go func() {
		if err := w.Run(ctx); err != nil && !isContextDone(err) {
			w.logger.Error("country watcher stopped", slog.String("error", err.Error()))
		}
	}()
}

func (w *CountryWatcher) processBatch(ctx context.Context) {
	ids, err := w.nodeRepo.ListPendingCountryLookups(ctx, w.cfg.BatchSize)
	if err != nil {
		w.logger.Error("failed to list pending country lookups", slog.String("error", err.Error()))
		return
	}
	if len(ids) == 0 {
		return
	}

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := w.nodeRepo.UpdateCountryInfo(ctx, id, w.lookupNode(ctx, id)); err != nil {
			w.logger.Error("failed to update country info", slog.String("node_id", id), slog.String("error", err.Error()))
		}
	}
}

func (w *CountryWatcher) lookupNode(ctx context.Context, nodeID string) *domain.CountryInfo {
	node, err := w.nodeRepo.FindByID(ctx, nodeID)
	if err != nil {
		w.logger.Warn("failed to load node for country lookup", slog.String("node_id", nodeID), slog.String("error", err.Error()))
		return w.failedInfo("failed to load node", 0)
	}

	host, err := w.nodeHost(ctx, node)
	if err != nil {
		w.logger.Warn("failed to determine node host", slog.String("node_id", nodeID), slog.String("error", err.Error()))
		return w.failedInfo(err.Error(), nextAttempts(node.CountryInfo))
	}

	info, err := w.resolver.Lookup(ctx, host)
	if err != nil {
		w.logger.Warn("country lookup failed", slog.String("node_id", nodeID), slog.String("host", host), slog.String("error", err.Error()))
		return w.failedInfo(err.Error(), nextAttempts(node.CountryInfo))
	}

	now := time.Now().UTC()
	info.LastLookupAt = &now
	info.NextRetryAt = nil
	info.Attempts = 0
	info.LastError = ""
	return &info
}

func nextAttempts(info *domain.CountryInfo) int {
	if info == nil {
		return 1
	}
	return info.Attempts + 1
}

func (w *CountryWatcher) nodeHost(ctx context.Context, node domain.Node) (string, error) {
	if node.IsSelf {
		return "", nil
	}

	if node.URL == "" {
		return "", fmt.Errorf("node has no url")
	}

	host := vless.ExtractIPFromVLESS(node.URL)
	if host == "" {
		return "", fmt.Errorf("failed to extract host from vless url")
	}

	if net.ParseIP(host) != nil {
		return host, nil
	}

	ip, err := w.resolver.ResolveHost(ctx, host)
	if err != nil {
		return "", fmt.Errorf("resolving host %q: %w", host, err)
	}
	return ip, nil
}

func (w *CountryWatcher) failedInfo(errMsg string, attempts int) *domain.CountryInfo {
	now := time.Now().UTC()
	retryAt := now.Add(w.cfg.RetryDelay)
	return &domain.CountryInfo{
		LastLookupAt: &now,
		NextRetryAt:  &retryAt,
		Attempts:     attempts,
		LastError:    errMsg,
	}
}

func isContextDone(err error) bool {
	if err == nil {
		return false
	}
	return err == context.Canceled || err == context.DeadlineExceeded
}
