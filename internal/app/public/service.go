package public

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"outless/internal/adapters/postgres"
	"outless/internal/domain"
)

// Service manages public VLESS sources import.
type Service struct {
	nodeRepo   domain.NodeRepository
	sourceRepo domain.PublicSourceRepository
	groupRepo  domain.GroupRepository
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
	NodeID    string         `json:"node_id"`
	URL       string         `json:"url"`
	Status    SyncNodeStatus `json:"status"`
	LatencyMS int64          `json:"latency_ms"`
	Error     string         `json:"error,omitempty"`
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
	Error      string                     `json:"error,omitempty"`
}

type SyncResult struct {
	SyncedAt                time.Time
	DeletedUnavailableCount int64
}

// NewService constructs a public sources service.
func NewService(
	nodeRepo domain.NodeRepository,
	sourceRepo domain.PublicSourceRepository,
	groupRepo domain.GroupRepository,
	logger *slog.Logger,
) *Service {
	return &Service{
		nodeRepo:   nodeRepo,
		sourceRepo: sourceRepo,
		groupRepo:  groupRepo,
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

// SyncGroup imports nodes for a group source URL and reports progress events.
func (s *Service) SyncGroup(ctx context.Context, groupID string, onEvent func(SyncEvent)) (SyncResult, error) {
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
	for _, rawURL := range vlessURLs {
		if err = ctx.Err(); err != nil {
			return SyncResult{}, err
		}

		nodeID := s.generateNodeID(rawURL)
		if onEvent != nil {
			onEvent(SyncEvent{
				NodeID: nodeID,
				URL:    rawURL,
				Status: SyncNodeStatusImporting,
			})
		}

		node := domain.Node{
			ID:      nodeID,
			URL:     rawURL,
			GroupID: groupID,
			Status:  domain.NodeStatusUnknown,
		}

		if err = s.nodeRepo.Upsert(ctx, node); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return SyncResult{}, err
			}
			if onEvent != nil {
				onEvent(SyncEvent{
					NodeID: nodeID,
					URL:    rawURL,
					Status: SyncNodeStatusError,
					Error:  err.Error(),
				})
			}
			continue
		}

		result := s.probeNodeQuick(ctx, rawURL, nodeID)
		if updateErr := s.nodeRepo.UpdateProbeResult(ctx, result); updateErr != nil {
			if errors.Is(updateErr, context.Canceled) || errors.Is(updateErr, context.DeadlineExceeded) {
				return SyncResult{}, updateErr
			}
			if onEvent != nil {
				onEvent(SyncEvent{
					NodeID: nodeID,
					URL:    rawURL,
					Status: SyncNodeStatusError,
					Error:  updateErr.Error(),
				})
			}
			continue
		}

		if onEvent != nil {
			status := SyncNodeStatusDone
			if result.Status != domain.NodeStatusHealthy {
				status = SyncNodeStatusUnavailable
			}
			onEvent(SyncEvent{
				NodeID:    nodeID,
				URL:       rawURL,
				Status:    status,
				LatencyMS: result.Latency.Milliseconds(),
			})
		}
	}

	syncedAt := time.Now().UTC()
	if err = s.groupRepo.UpdateSyncedAt(ctx, groupID, syncedAt); err != nil {
		return SyncResult{}, fmt.Errorf("updating group sync timestamp: %w", err)
	}

	var deletedUnavailable int64
	if group.AutoDeleteUnavailable {
		deletedUnavailable, err = s.nodeRepo.DeleteUnavailableByGroup(ctx, groupID)
		if err != nil {
			return SyncResult{}, fmt.Errorf("auto-deleting unavailable nodes: %w", err)
		}
	}

	return SyncResult{
		SyncedAt:                syncedAt,
		DeletedUnavailableCount: deletedUnavailable,
	}, nil
}

// ProbeUnavailableGroup probes all non-healthy nodes in a group and emits lifecycle events.
func (s *Service) ProbeUnavailableGroup(ctx context.Context, groupID string, onEvent func(ProbeUnavailableEvent)) (int, error) {
	if _, err := s.groupRepo.FindByID(ctx, groupID); err != nil {
		return 0, fmt.Errorf("finding group: %w", err)
	}

	nodes, err := s.nodeRepo.ListNonHealthyByGroup(ctx, groupID)
	if err != nil {
		return 0, fmt.Errorf("listing non-healthy nodes: %w", err)
	}

	for _, node := range nodes {
		if onEvent != nil {
			onEvent(ProbeUnavailableEvent{
				NodeID: node.ID,
				URL:    node.URL,
				Status: ProbeUnavailableNodeStatusQueued,
			})
		}
	}

	probed := 0
	for _, node := range nodes {
		if err := ctx.Err(); err != nil {
			return probed, err
		}

		if onEvent != nil {
			onEvent(ProbeUnavailableEvent{
				NodeID: node.ID,
				URL:    node.URL,
				Status: ProbeUnavailableNodeStatusProbing,
			})
		}

		result := s.probeNodeQuick(ctx, node.URL, node.ID)
		if saveErr := s.nodeRepo.UpdateProbeResult(ctx, result); saveErr != nil {
			if errors.Is(saveErr, context.Canceled) || errors.Is(saveErr, context.DeadlineExceeded) {
				return probed, saveErr
			}
			if onEvent != nil {
				onEvent(ProbeUnavailableEvent{
					NodeID: node.ID,
					URL:    node.URL,
					Status: ProbeUnavailableNodeStatusError,
					Error:  saveErr.Error(),
				})
			}
			continue
		}

		probed++
		if onEvent != nil {
			onEvent(ProbeUnavailableEvent{
				NodeID:     node.ID,
				URL:        node.URL,
				Status:     ProbeUnavailableNodeStatusReady,
				LatencyMS:  result.Latency.Milliseconds(),
				NodeStatus: string(result.Status),
			})
		}
	}

	return probed, nil
}

// CountUnavailableByGroup returns number of non-healthy nodes in group.
func (s *Service) CountUnavailableByGroup(ctx context.Context, groupID string) (int, error) {
	nodes, err := s.nodeRepo.ListNonHealthyByGroup(ctx, groupID)
	if err != nil {
		return 0, fmt.Errorf("listing non-healthy nodes: %w", err)
	}
	return len(nodes), nil
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
func (s *Service) parseVLESSLines(content string) []string {
	lines := strings.Split(content, "\n")
	urls := make([]string, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "vless://") {
			urls = append(urls, line)
		}
	}

	return urls
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

func (s *Service) probeNodeQuick(ctx context.Context, rawURL string, nodeID string) domain.ProbeResult {
	start := time.Now()
	result := domain.ProbeResult{
		NodeID:    nodeID,
		Status:    domain.NodeStatusUnhealthy,
		CheckedAt: time.Now().UTC(),
	}

	addr, err := probeAddressFromVLESS(rawURL)
	if err != nil {
		return result
	}

	dialer := net.Dialer{Timeout: 4 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return result
	}
	_ = conn.Close()

	result.Status = domain.NodeStatusHealthy
	result.Latency = time.Since(start)
	return result
}

func probeAddressFromVLESS(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("vless url host is empty")
	}
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	return net.JoinHostPort(host, port), nil
}

// generateNodeID creates a deterministic short ID from URL via SHA-256.
func (s *Service) generateNodeID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return "node_" + hex.EncodeToString(hash[:8])
}
