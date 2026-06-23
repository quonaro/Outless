package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"outless/internal/domain"
)

// TrafficCollector periodically snapshots the sing-box runtime, aggregates
// per-token traffic deltas, persists them, and enforces quotas.
type TrafficCollector struct {
	runtimeCtrl domain.RuntimeController
	trafficRepo domain.TrafficRepository
	tokenRepo   domain.TokenRepository
	logger      *slog.Logger
	interval    time.Duration
	stopCh      chan struct{}
	stoppedCh   chan struct{}

	mu                sync.Mutex
	lastSeen          map[string]connectionState
	lastUploadTotal   int64
	lastDownloadTotal int64
}

type connectionState struct {
	upload   int64
	download int64
}

// NewTrafficCollector constructs a traffic collector with a 30s default interval.
func NewTrafficCollector(
	runtimeCtrl domain.RuntimeController,
	trafficRepo domain.TrafficRepository,
	tokenRepo domain.TokenRepository,
	logger *slog.Logger,
) *TrafficCollector {
	return &TrafficCollector{
		runtimeCtrl: runtimeCtrl,
		trafficRepo: trafficRepo,
		tokenRepo:   tokenRepo,
		logger:      logger,
		interval:    30 * time.Second,
		stopCh:      make(chan struct{}),
		stoppedCh:   make(chan struct{}),
		lastSeen:    make(map[string]connectionState),
	}
}

// WithInterval sets a custom collection interval (useful for testing).
func (s *TrafficCollector) WithInterval(d time.Duration) *TrafficCollector {
	s.interval = d
	return s
}

// Start begins the periodic collection goroutine.
func (s *TrafficCollector) Start(ctx context.Context) error {
	s.logger.Info("traffic collector started", slog.Duration("interval", s.interval))
	go s.loop(ctx)
	return nil
}

// Stop signals the collector loop to stop and waits for it to finish.
func (s *TrafficCollector) Stop() error {
	close(s.stopCh)
	<-s.stoppedCh
	return nil
}

func (s *TrafficCollector) loop(ctx context.Context) {
	defer close(s.stoppedCh)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("traffic collector stopping (context done)")
			return
		case <-s.stopCh:
			s.logger.Info("traffic collector stopped")
			return
		case <-ticker.C:
			if err := s.collect(ctx); err != nil {
				s.logger.Error("traffic collection failed", slog.String("error", err.Error()))
			}
		}
	}
}

type delta struct {
	upload   int64
	download int64
}

func (s *TrafficCollector) collect(ctx context.Context) error {
	snap := s.runtimeCtrl.TrafficSnapshot()
	if snap == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Detect sing-box restart (counters dropped to zero or lower).
	if snap.UploadTotal < s.lastUploadTotal || snap.DownloadTotal < s.lastDownloadTotal {
		s.lastSeen = make(map[string]connectionState)
	}

	currentIDs, tokenDeltas, nodeDeltas, inboundDeltas, domainDeltas := s.aggregateDeltas(snap)

	// Remove closed connections from lastSeen.
	for id := range s.lastSeen {
		if _, ok := currentIDs[id]; !ok {
			delete(s.lastSeen, id)
		}
	}

	s.lastUploadTotal = snap.UploadTotal
	s.lastDownloadTotal = snap.DownloadTotal

	now := time.Now().UTC()
	s.persistTokenDeltas(ctx, now, tokenDeltas)
	s.persistNodeDeltas(ctx, now, nodeDeltas)
	s.persistInboundDeltas(ctx, now, inboundDeltas)
	s.persistDomainDeltas(ctx, now, domainDeltas)

	// Enforce quotas.
	if err := s.enforceQuotas(ctx, now); err != nil {
		s.logger.Error("quota enforcement failed", slog.String("error", err.Error()))
	}

	return nil
}

type domainDeltaKey struct {
	tokenID string
	domain  string
}

func (s *TrafficCollector) aggregateDeltas(
	snap *domain.TrafficSnapshot,
) (map[string]struct{}, map[string]delta, map[string]delta, map[string]delta, map[domainDeltaKey]delta) {
	currentIDs := make(map[string]struct{}, len(snap.Connections))
	tokenDeltas := make(map[string]delta)
	nodeDeltas := make(map[string]delta)
	inboundDeltas := make(map[string]delta)
	domainDeltas := make(map[domainDeltaKey]delta)

	for _, conn := range snap.Connections {
		currentIDs[conn.ID] = struct{}{}
		last := s.lastSeen[conn.ID]
		du := conn.Upload - last.upload
		dd := conn.Download - last.download
		if du < 0 {
			du = conn.Upload
		}
		if dd < 0 {
			dd = conn.Download
		}

		if tokenID := parseTokenID(conn.User); tokenID != "" {
			st := tokenDeltas[tokenID]
			st.upload += du
			st.download += dd
			tokenDeltas[tokenID] = st

			if conn.Domain != "" {
				dk := domainDeltaKey{tokenID: tokenID, domain: conn.Domain}
				dst := domainDeltas[dk]
				dst.upload += du
				dst.download += dd
				domainDeltas[dk] = dst
			}
		}
		if conn.NodeID != "" {
			st := nodeDeltas[conn.NodeID]
			st.upload += du
			st.download += dd
			nodeDeltas[conn.NodeID] = st
		}
		if conn.Inbound != "" {
			st := inboundDeltas[conn.Inbound]
			st.upload += du
			st.download += dd
			inboundDeltas[conn.Inbound] = st
		}

		s.lastSeen[conn.ID] = connectionState{
			upload:   conn.Upload,
			download: conn.Download,
		}
	}

	return currentIDs, tokenDeltas, nodeDeltas, inboundDeltas, domainDeltas
}

func (s *TrafficCollector) persistTokenDeltas(
	ctx context.Context,
	now time.Time,
	deltas map[string]delta,
) {
	for id, d := range deltas {
		if d.upload == 0 && d.download == 0 {
			continue
		}
		dayStart := periodStart("day", now)
		if err := s.upsertDelta(ctx, id, "day", dayStart, d.upload, d.download); err != nil {
			s.logger.Error("failed to record daily usage", slog.String("token_id", id), slog.String("error", err.Error()))
		}
		monthStart := periodStart("month", now)
		if err := s.upsertDelta(ctx, id, "month", monthStart, d.upload, d.download); err != nil {
			s.logger.Error("failed to record monthly usage", slog.String("token_id", id), slog.String("error", err.Error()))
		}
		if err := s.tokenRepo.RecordTokenConnection(ctx, id, d.upload, d.download, now); err != nil {
			s.logger.Error("failed to record token connection", slog.String("token_id", id), slog.String("error", err.Error()))
		}
	}
}

func (s *TrafficCollector) persistNodeDeltas(
	ctx context.Context,
	now time.Time,
	deltas map[string]delta,
) {
	for id, d := range deltas {
		if d.upload == 0 && d.download == 0 {
			continue
		}
		dayStart := periodStart("day", now)
		if err := s.upsertNodeDelta(ctx, id, "day", dayStart, d.upload, d.download); err != nil {
			s.logger.Error("failed to record daily node usage", slog.String("node_id", id), slog.String("error", err.Error()))
		}
		monthStart := periodStart("month", now)
		if err := s.upsertNodeDelta(ctx, id, "month", monthStart, d.upload, d.download); err != nil {
			s.logger.Error("failed to record monthly node usage", slog.String("node_id", id), slog.String("error", err.Error()))
		}
	}
}

func (s *TrafficCollector) persistInboundDeltas(
	ctx context.Context,
	now time.Time,
	deltas map[string]delta,
) {
	for id, d := range deltas {
		if d.upload == 0 && d.download == 0 {
			continue
		}
		dayStart := periodStart("day", now)
		if err := s.upsertInboundDelta(ctx, id, "day", dayStart, d.upload, d.download); err != nil {
			s.logger.Error("failed to record daily inbound usage", slog.String("inbound", id), slog.String("error", err.Error()))
		}
		monthStart := periodStart("month", now)
		if err := s.upsertInboundDelta(ctx, id, "month", monthStart, d.upload, d.download); err != nil {
			s.logger.Error("failed to record monthly inbound usage", slog.String("inbound", id), slog.String("error", err.Error()))
		}
	}
}

func (s *TrafficCollector) persistDomainDeltas(
	ctx context.Context,
	now time.Time,
	deltas map[domainDeltaKey]delta,
) {
	for key, d := range deltas {
		if d.upload == 0 && d.download == 0 {
			continue
		}
		dayStart := periodStart("day", now)
		if err := s.upsertDomainDelta(
			ctx, key.tokenID, key.domain, "day", dayStart, d.upload, d.download,
		); err != nil {
			s.logger.Error("failed to record daily domain usage",
				slog.String("token_id", key.tokenID),
				slog.String("domain", key.domain),
				slog.String("error", err.Error()),
			)
		}
		monthStart := periodStart("month", now)
		if err := s.upsertDomainDelta(
			ctx, key.tokenID, key.domain, "month", monthStart, d.upload, d.download,
		); err != nil {
			s.logger.Error("failed to record monthly domain usage",
				slog.String("token_id", key.tokenID),
				slog.String("domain", key.domain),
				slog.String("error", err.Error()),
			)
		}
	}
}

func (s *TrafficCollector) upsertDomainDelta(
	ctx context.Context,
	tokenID string,
	domainName string,
	periodType string,
	periodStart int64,
	uploadDelta int64,
	downloadDelta int64,
) error {
	usage, err := s.trafficRepo.GetDomainUsage(ctx, tokenID, domainName, periodType, time.Unix(0, periodStart).UTC())
	if err != nil {
		usage = domain.DomainUsage{
			TokenID:     tokenID,
			Domain:      domainName,
			PeriodType:  periodType,
			PeriodStart: time.Unix(0, periodStart).UTC(),
		}
	}
	usage.UploadBytes += uploadDelta
	usage.DownloadBytes += downloadDelta
	usage.UpdatedAt = time.Now().UTC()
	return s.trafficRepo.RecordDomainUsage(ctx, usage)
}

func (s *TrafficCollector) upsertDelta(
	ctx context.Context,
	tokenID, periodType string,
	periodStart, uploadDelta, downloadDelta int64,
) error {
	usage, err := s.trafficRepo.GetUsage(ctx, tokenID, periodType, time.Unix(0, periodStart).UTC())
	if err != nil {
		return fmt.Errorf("getting usage for upsert: %w", err)
	}

	usage.TokenID = tokenID
	usage.PeriodType = periodType
	usage.PeriodStart = time.Unix(0, periodStart).UTC()
	usage.UploadBytes += uploadDelta
	usage.DownloadBytes += downloadDelta
	usage.UpdatedAt = time.Now().UTC()

	if err := s.trafficRepo.RecordUsage(ctx, usage); err != nil {
		return fmt.Errorf("recording usage: %w", err)
	}
	return nil
}

func (s *TrafficCollector) upsertNodeDelta(
	ctx context.Context,
	nodeID, periodType string,
	periodStart, uploadDelta, downloadDelta int64,
) error {
	usage, err := s.trafficRepo.GetNodeUsage(ctx, nodeID, periodType, time.Unix(0, periodStart).UTC())
	if err != nil {
		return fmt.Errorf("getting node usage for upsert: %w", err)
	}
	usage.NodeID = nodeID
	usage.PeriodType = periodType
	usage.PeriodStart = time.Unix(0, periodStart).UTC()
	usage.UploadBytes += uploadDelta
	usage.DownloadBytes += downloadDelta
	usage.UpdatedAt = time.Now().UTC()
	if err := s.trafficRepo.RecordNodeUsage(ctx, usage); err != nil {
		return fmt.Errorf("recording node usage: %w", err)
	}
	return nil
}

func (s *TrafficCollector) upsertInboundDelta(
	ctx context.Context,
	inboundTag, periodType string,
	periodStart, uploadDelta, downloadDelta int64,
) error {
	usage, err := s.trafficRepo.GetInboundUsage(ctx, inboundTag, periodType, time.Unix(0, periodStart).UTC())
	if err != nil {
		return fmt.Errorf("getting inbound usage for upsert: %w", err)
	}
	usage.InboundTag = inboundTag
	usage.PeriodType = periodType
	usage.PeriodStart = time.Unix(0, periodStart).UTC()
	usage.UploadBytes += uploadDelta
	usage.DownloadBytes += downloadDelta
	usage.UpdatedAt = time.Now().UTC()
	if err := s.trafficRepo.RecordInboundUsage(ctx, usage); err != nil {
		return fmt.Errorf("recording inbound usage: %w", err)
	}
	return nil
}

func (s *TrafficCollector) enforceQuotas(ctx context.Context, now time.Time) error {
	tokens, err := s.tokenRepo.ListActive(ctx, now)
	if err != nil {
		return fmt.Errorf("listing active tokens for quota check: %w", err)
	}

	for _, token := range tokens {
		if token.QuotaBytes == nil || *token.QuotaBytes <= 0 || token.QuotaPeriod == "" {
			continue
		}

		start := periodStart(token.QuotaPeriod, now)
		usage, err := s.trafficRepo.GetUsage(ctx, token.ID, token.QuotaPeriod, time.Unix(0, start).UTC())
		if err != nil {
			s.logger.Error("failed to get usage for quota check",
				slog.String("token_id", token.ID), slog.String("error", err.Error()))
			continue
		}

		total := usage.UploadBytes + usage.DownloadBytes
		if total > *token.QuotaBytes {
			if err := s.tokenRepo.Deactivate(ctx, token.ID); err != nil {
				s.logger.Error("failed to deactivate token on quota exceeded",
					slog.String("token_id", token.ID), slog.String("error", err.Error()))
				continue
			}
			s.logger.Info("token deactivated due to quota exceeded",
				slog.String("token_id", token.ID),
				slog.Int64("usage", total),
				slog.Int64("quota", *token.QuotaBytes))
		}
	}

	return nil
}

// parseTokenID extracts the token identifier from a sing-box inbound user name.
// Expected format: "t-<tokenID>-n-<nodeID>".
func parseTokenID(user string) string {
	parts := strings.Split(user, "-")
	if len(parts) < 4 || parts[0] != "t" || parts[2] != "n" {
		return ""
	}
	return parts[1]
}

// periodStart returns the period boundary for the given period type.
func periodStart(periodType string, now time.Time) int64 {
	now = now.UTC()
	switch periodType {
	case "day":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).UnixNano()
	case "month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).UnixNano()
	}
	return now.UnixNano()
}
