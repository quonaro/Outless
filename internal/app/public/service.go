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
)

// Service manages public VLESS sources import.
type Service struct {
	nodeRepo   domain.NodeRepository
	sourceRepo domain.PublicSourceRepository
	groupRepo  domain.GroupRepository
	httpClient *http.Client
	logger     *slog.Logger
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
		if err := s.ImportNodes(ctx, source.ID); err != nil {
			s.logger.Error("failed to import from source", slog.String("source_id", source.ID), slog.String("error", err.Error()))
			continue
		}
		total++
	}

	s.logger.Info("public sources import completed", slog.Int("sources_processed", total))
	return nil
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
		nodeID := s.generateNodeID(url)

		node := domain.Node{
			ID:      nodeID,
			URL:     url,
			GroupID: groupID,
			Status:  domain.NodeStatusUnknown,
			Country: "",
			Latency: 0,
		}

		if err := s.nodeRepo.Create(ctx, node); err != nil {
			if isDuplicateKeyError(err) {
				continue
			}
			s.logger.Warn("failed to import node", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			continue
		}
		created++
	}

	return created, nil
}

// generateNodeID creates a deterministic short ID from URL via SHA-256.
func (s *Service) generateNodeID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return "node_" + hex.EncodeToString(hash[:8])
}

// isDuplicateKeyError reports whether err indicates a unique/primary key violation.
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, domain.ErrDuplicateNode) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "unique_violation") ||
		strings.Contains(msg, "sqlstate 23505")
}
