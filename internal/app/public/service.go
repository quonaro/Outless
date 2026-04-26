package public

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"outless/internal/adapters/postgres"
	"outless/internal/domain"
	"outless/pkg/vless"
)

// syncLoadBatchSize limits rows per INSERT for Load to keep queries and WS fan-out bounded.
const syncLoadBatchSize = 500

// Service manages public VLESS sources import.
type Service struct {
	nodeRepo   domain.NodeRepository
	sourceRepo domain.PublicSourceRepository
	groupRepo  domain.GroupRepository
	geoip      domain.GeoIPResolver
	httpClient *http.Client
	logger     *slog.Logger
}

// SyncNodeStatus describes per-node sync state for SSE streaming.
type SyncNodeStatus string

const (
	SyncNodeStatusImporting   SyncNodeStatus = "importing"
	SyncNodeStatusDone        SyncNodeStatus = "done"
	SyncNodeStatusUnavailable SyncNodeStatus = "unavailable"
	SyncNodeStatusError       SyncNodeStatus = "error"
)

// SyncEvent is emitted for each node while group sync is running.
type SyncEvent struct {
	NodeID     string         `json:"node_id"`
	URL        string         `json:"url"`
	Status     SyncNodeStatus `json:"status"`
	AddedTotal int            `json:"added_total,omitempty"`
	Error      string         `json:"error,omitempty"`
}

type SyncResult struct {
	SyncedAt   time.Time
	AddedCount int
}

// NewService constructs a public sources service.
func NewService(
	nodeRepo domain.NodeRepository,
	sourceRepo domain.PublicSourceRepository,
	groupRepo domain.GroupRepository,
	geoip domain.GeoIPResolver,
	logger *slog.Logger,
) *Service {
	return &Service{
		nodeRepo:   nodeRepo,
		sourceRepo: sourceRepo,
		groupRepo:  groupRepo,
		geoip:      geoip,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

// ImportNodes fetches and imports nodes from a public source.
func (s *Service) ImportNodes(ctx context.Context, sourceID string) error {
	source, err := s.sourceRepo.FindByID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("finding source: %w", err)
	}

	content, err := s.fetchSource(ctx, source.URL)
	if err != nil {
		return fmt.Errorf("fetching source %s: %w", source.URL, err)
	}

	vlessURLs := s.parseVLESSLines(content)
	if len(vlessURLs) == 0 {
		s.logger.Info("no VLESS URLs found in source", slog.String("source_id", sourceID))
		return nil
	}

	count, err := s.importURLs(ctx, vlessURLs, source.GroupID)
	if err != nil {
		return fmt.Errorf("importing URLs: %w", err)
	}

	now := time.Now().UTC()
	source.LastFetchedAt = &now
	if err := s.sourceRepo.Update(ctx, source); err != nil {
		s.logger.Warn("failed to update last_fetched_at", slog.String("source_id", sourceID), slog.String("error", err.Error()))
	}

	s.logger.Info("nodes imported from source", slog.String("source_id", sourceID), slog.Int("count", count))
	return nil
}

// ImportAll imports nodes from all public sources.
func (s *Service) ImportAll(ctx context.Context) error {
	sources, err := s.sourceRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("listing sources: %w", err)
	}

	total := 0
	for _, source := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.ImportNodes(ctx, source.ID); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			s.logger.Error("failed to import from source", slog.String("source_id", source.ID), slog.String("error", err.Error()))
			continue
		}
		total++
	}

	s.logger.Info("public sources import completed", slog.Int("sources_processed", total))
	return nil
}

// SyncGroup loads nodes for a group source URL and reports progress events.
// It only imports/upserts nodes from source without probing their health.
func (s *Service) SyncGroup(ctx context.Context, groupID string, onTotal func(int), onEvent func(SyncEvent)) (SyncResult, error) {
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return SyncResult{}, fmt.Errorf("finding group: %w", err)
	}
	if strings.TrimSpace(group.SourceURL) == "" {
		return SyncResult{}, fmt.Errorf("group has no source url")
	}

	content, err := s.fetchSource(ctx, group.SourceURL)
	if err != nil {
		return SyncResult{}, fmt.Errorf("fetching source %s: %w", group.SourceURL, err)
	}

	vlessURLs := s.parseVLESSLines(content)
	uniqueURLs := make([]string, 0, len(vlessURLs))
	seen := make(map[string]struct{}, len(vlessURLs))
	for _, raw := range vlessURLs {
		k := strings.TrimSpace(raw)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		uniqueURLs = append(uniqueURLs, k)
	}
	if onTotal != nil {
		onTotal(len(uniqueURLs))
	}
	addedTotal := 0
	for start := 0; start < len(uniqueURLs); start += syncLoadBatchSize {
		if err = ctx.Err(); err != nil {
			return SyncResult{}, err
		}

		end := start + syncLoadBatchSize
		if end > len(uniqueURLs) {
			end = len(uniqueURLs)
		}
		chunk := uniqueURLs[start:end]

		nodes := make([]domain.Node, len(chunk))
		for i, rawURL := range chunk {
			country := s.resolveCountry(ctx, rawURL)
			nodes[i] = domain.Node{
				ID:      s.generateNodeID(rawURL),
				URL:     rawURL,
				GroupID: groupID,
				Country: country,
			}
		}

		if onEvent != nil {
			for _, rawURL := range chunk {
				onEvent(SyncEvent{
					NodeID: s.generateNodeID(rawURL),
					URL:    rawURL,
					Status: SyncNodeStatusImporting,
				})
			}
		}

		insertedIDs, bulkErr := s.nodeRepo.BulkCreateIfAbsent(ctx, nodes)
		if bulkErr != nil {
			err = bulkErr
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return SyncResult{}, err
			}
			for _, rawURL := range chunk {
				nodeID := s.generateNodeID(rawURL)
				if onEvent != nil {
					onEvent(SyncEvent{
						NodeID:     nodeID,
						URL:        rawURL,
						Status:     SyncNodeStatusError,
						AddedTotal: addedTotal,
						Error:      err.Error(),
					})
				}
			}
			continue
		}

		inserted := make(map[string]struct{}, len(insertedIDs))
		for _, id := range insertedIDs {
			inserted[id] = struct{}{}
		}

		for _, rawURL := range chunk {
			nodeID := s.generateNodeID(rawURL)
			if _, ok := inserted[nodeID]; ok {
				addedTotal++
			}
			if onEvent != nil {
				onEvent(SyncEvent{
					NodeID:     nodeID,
					URL:        rawURL,
					Status:     SyncNodeStatusDone,
					AddedTotal: addedTotal,
				})
			}
		}
	}

	syncedAt := time.Now().UTC()
	if err = s.groupRepo.UpdateSyncedAt(ctx, groupID, syncedAt); err != nil {
		return SyncResult{}, fmt.Errorf("updating group sync timestamp: %w", err)
	}

	return SyncResult{
		SyncedAt:   syncedAt,
		AddedCount: addedTotal,
	}, nil
}

// EnsurePublicGroup creates the "Public" group if it doesn't exist.
func (s *Service) EnsurePublicGroup(ctx context.Context) (string, error) {
	groups, err := s.groupRepo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("listing groups: %w", err)
	}

	for _, g := range groups {
		if g.Name == "Public" {
			return g.ID, nil
		}
	}

	id, err := postgres.GenerateGroupID()
	if err != nil {
		return "", fmt.Errorf("generating group id: %w", err)
	}

	group := domain.Group{
		ID:        id,
		Name:      "Public",
		CreatedAt: time.Now().UTC(),
	}

	if err := s.groupRepo.Create(ctx, group); err != nil {
		return "", fmt.Errorf("creating public group: %w", err)
	}

	s.logger.Info("public group created", slog.String("id", id))
	return id, nil
}

// fetchSource downloads content from URL.
func (s *Service) fetchSource(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}

	return string(body), nil
}

// parseVLESSLines extracts VLESS URLs from content (one per line).
// Filters out invalid URLs that would cause Xray config errors.
func (s *Service) parseVLESSLines(content string) []string {
	lines := strings.Split(content, "\n")
	urls := make([]string, 0)
	skipped := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "vless://") {
			continue
		}

		// Validate VLESS URL to prevent Xray config errors
		if _, err := vless.ParseURL(line); err != nil {
			s.logger.Debug("skipping invalid VLESS URL",
				slog.String("error", err.Error()),
				slog.String("url_prefix", line[:min(100, len(line))]))
			skipped++
			continue
		}

		urls = append(urls, line)
	}

	if skipped > 0 {
		s.logger.Info("filtered invalid VLESS URLs during import", slog.Int("skipped", skipped))
	}

	return urls
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// importURLs adds VLESS URLs as nodes to the database.
func (s *Service) importURLs(ctx context.Context, urls []string, groupID string) (int, error) {
	created := 0

	for _, url := range urls {
		if err := ctx.Err(); err != nil {
			return created, err
		}
		nodeID := s.generateNodeID(url)
		country := s.resolveCountry(ctx, url)

		node := domain.Node{
			ID:      nodeID,
			URL:     url,
			GroupID: groupID,
			Country: country,
		}

		createdNow, err := s.nodeRepo.CreateIfAbsent(ctx, node)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return created, err
			}
			s.logger.Warn("failed to import node", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			continue
		}
		if createdNow {
			created++
		}
	}

	return created, nil
}

// generateNodeID creates a deterministic short ID from URL via SHA-256.
func (s *Service) generateNodeID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return "node_" + hex.EncodeToString(hash[:8])
}

// resolveCountry determines the country for a VLESS URL using GeoIP.
// Returns empty string if geoip is not configured or lookup fails.
func (s *Service) resolveCountry(ctx context.Context, vlessURL string) string {
	if s.geoip == nil {
		return ""
	}

	ip := vless.ExtractIPFromVLESS(vlessURL)
	if ip == "" {
		return ""
	}

	country, err := s.geoip.LookupCountry(ctx, ip)
	if err != nil {
		s.logger.Debug("geoip lookup failed",
			slog.String("ip", ip),
			slog.String("error", err.Error()))
		return ""
	}

	return country
}
