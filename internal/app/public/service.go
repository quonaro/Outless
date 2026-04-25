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
	"sync"
	"sync/atomic"
	"time"

	"outless/internal/adapters/postgres"
	"outless/internal/app/nodeprobe"
	"outless/internal/domain"
	"outless/pkg/vless"
)

// syncLoadBatchSize limits rows per INSERT for Load to keep queries and WS fan-out bounded.
const syncLoadBatchSize = 500

// probeWorkerPoolSize caps concurrent probes for Check all (TCP / Xray).
const probeWorkerPoolSize = 32

// Service manages public VLESS sources import.
type Service struct {
	nodeRepo   domain.NodeRepository
	sourceRepo domain.PublicSourceRepository
	groupRepo  domain.GroupRepository
	engine     domain.ProxyEngine
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
	LatencyMS  int64          `json:"latency_ms"`
	AddedTotal int            `json:"added_total,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// ProbeUnavailableNodeStatus describes per-node status while probing unavailable nodes.
type ProbeUnavailableNodeStatus string

const (
	ProbeUnavailableNodeStatusQueued  ProbeUnavailableNodeStatus = "queued"
	ProbeUnavailableNodeStatusProbing ProbeUnavailableNodeStatus = "probing"
	ProbeUnavailableNodeStatusReady   ProbeUnavailableNodeStatus = "ready"
	ProbeUnavailableNodeStatusError   ProbeUnavailableNodeStatus = "error"
)

// ProbeUnavailableEvent is emitted for each unavailable node probe lifecycle.
type ProbeUnavailableEvent struct {
	NodeID     string                     `json:"node_id"`
	URL        string                     `json:"url"`
	Status     ProbeUnavailableNodeStatus `json:"status"`
	LatencyMS  int64                      `json:"latency_ms"`
	NodeStatus string                     `json:"node_status,omitempty"`
	Country    string                     `json:"country,omitempty"`
	Error      string                     `json:"error,omitempty"`
}

type SyncResult struct {
	SyncedAt                time.Time
	DeletedUnavailableCount int64
	AddedCount              int
}

// NewService constructs a public sources service.
func NewService(
	nodeRepo domain.NodeRepository,
	sourceRepo domain.PublicSourceRepository,
	groupRepo domain.GroupRepository,
	engine domain.ProxyEngine,
	logger *slog.Logger,
) *Service {
	return &Service{
		nodeRepo:   nodeRepo,
		sourceRepo: sourceRepo,
		groupRepo:  groupRepo,
		engine:     engine,
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
			nodes[i] = domain.Node{
				ID:      s.generateNodeID(rawURL),
				URL:     rawURL,
				GroupID: groupID,
				Status:  domain.NodeStatusUnknown,
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
					LatencyMS:  0,
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
		SyncedAt:                syncedAt,
		DeletedUnavailableCount: 0,
		AddedCount:              addedTotal,
	}, nil
}

// ProbeUnavailableGroup probes all nodes in a group and emits lifecycle events.
func (s *Service) ProbeUnavailableGroup(ctx context.Context, groupID string, statuses []domain.NodeStatus, onEvent func(ProbeUnavailableEvent)) (int, error) {
	if _, err := s.groupRepo.FindByID(ctx, groupID); err != nil {
		return 0, fmt.Errorf("finding group: %w", err)
	}

	nodes, err := s.nodeRepo.ListByGroup(ctx, groupID)
	if err != nil {
		return 0, fmt.Errorf("listing nodes by group: %w", err)
	}
	nodes = filterNodesByStatuses(nodes, statuses)

	for _, node := range nodes {
		if onEvent != nil {
			onEvent(ProbeUnavailableEvent{
				NodeID: node.ID,
				URL:    node.URL,
				Status: ProbeUnavailableNodeStatusQueued,
			})
		}
	}

	if len(nodes) == 0 {
		return 0, nil
	}

	jobs := make(chan domain.Node, len(nodes))
	for _, node := range nodes {
		jobs <- node
	}
	close(jobs)

	workers := probeWorkerPoolSize
	if workers > len(nodes) {
		workers = len(nodes)
	}

	var wg sync.WaitGroup
	var probed atomic.Int64

	runProbe := func(node domain.Node) {
		if onEvent != nil {
			onEvent(ProbeUnavailableEvent{
				NodeID: node.ID,
				URL:    node.URL,
				Status: ProbeUnavailableNodeStatusProbing,
			})
		}

		result := nodeprobe.ProbeWithEngine(ctx, s.engine, node)
		if saveErr := s.nodeRepo.UpdateProbeResult(ctx, result); saveErr != nil {
			if errors.Is(saveErr, context.Canceled) || errors.Is(saveErr, context.DeadlineExceeded) {
				return
			}
			if onEvent != nil {
				onEvent(ProbeUnavailableEvent{
					NodeID: node.ID,
					URL:    node.URL,
					Status: ProbeUnavailableNodeStatusError,
					Error:  saveErr.Error(),
				})
			}
			return
		}

		probed.Add(1)
		if onEvent != nil {
			onEvent(ProbeUnavailableEvent{
				NodeID:     node.ID,
				URL:        node.URL,
				Status:     ProbeUnavailableNodeStatusReady,
				LatencyMS:  result.Latency.Milliseconds(),
				NodeStatus: string(result.Status),
				Country:    result.Country,
			})
		}
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for node := range jobs {
				if err := ctx.Err(); err != nil {
					return
				}
				runProbe(node)
			}
		}()
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return int(probed.Load()), err
	}
	return int(probed.Load()), nil
}

// CountUnavailableByGroup returns number of nodes in group.
func (s *Service) CountUnavailableByGroup(ctx context.Context, groupID string, statuses []domain.NodeStatus) (int, error) {
	nodes, err := s.nodeRepo.ListByGroup(ctx, groupID)
	if err != nil {
		return 0, fmt.Errorf("listing nodes by group: %w", err)
	}
	return len(filterNodesByStatuses(nodes, statuses)), nil
}

func filterNodesByStatuses(nodes []domain.Node, statuses []domain.NodeStatus) []domain.Node {
	if len(statuses) == 0 {
		return nodes
	}
	allowed := make(map[domain.NodeStatus]struct{}, len(statuses))
	for _, st := range statuses {
		allowed[st] = struct{}{}
	}
	out := make([]domain.Node, 0, len(nodes))
	for _, node := range nodes {
		if _, ok := allowed[node.Status]; ok {
			out = append(out, node)
		}
	}
	return out
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

		node := domain.Node{
			ID:      nodeID,
			URL:     url,
			GroupID: groupID,
			Status:  domain.NodeStatusUnknown,
			Country: "",
			Latency: 0,
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
