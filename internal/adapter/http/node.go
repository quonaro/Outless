package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/country"
	"outless/internal/domain"
)

type NodeManagementHandler struct {
	nodeRepo        domain.NodeRepository
	groupRepo       domain.GroupRepository
	runtime         RuntimeController
	countryResolver *country.Resolver
	logger          *slog.Logger
}

func NewNodeManagementHandler(
	nodeRepo domain.NodeRepository,
	groupRepo domain.GroupRepository,
	runtime RuntimeController,
	countryResolver *country.Resolver,
	logger *slog.Logger,
) *NodeManagementHandler {
	return &NodeManagementHandler{
		nodeRepo:        nodeRepo,
		groupRepo:       groupRepo,
		runtime:         runtime,
		countryResolver: countryResolver,
		logger:          logger,
	}
}

type ListNodesOutput struct {
	Body struct {
		Nodes      []NodeItem `json:"nodes"`
		NextOffset *int       `json:"next_offset,omitempty"`
		HasMore    bool       `json:"has_more"`
	}
}

type ListNodesInput struct {
	Limit   int    `query:"limit"`
	Offset  int    `query:"offset"`
	GroupID string `query:"group_id"`
}

type DeleteNodeInput struct {
	ID string `path:"id" required:"true"`
}

type GetNodeInput struct {
	ID string `path:"id" required:"true"`
}

type GetNodeOutput struct {
	Body NodeItem `json:"node"`
}

type NodeItem struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	GroupIDs    []string `json:"group_ids"`
	Country     string   `json:"country"`
	CountryCode string   `json:"country_code"`
	CountryName string   `json:"country_name"`
	CountryFlag string   `json:"country_flag"`
	IsSelf      bool     `json:"is_self"`
	ExpiresAt   *string  `json:"expires_at,omitempty"`
}

func (h *NodeManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/nodes", h.CreateNode)
	huma.Get(api, "/v1/nodes", h.ListNodes)
	huma.Get(api, "/v1/nodes/{id}", h.GetNode)
	huma.Patch(api, "/v1/nodes/{id}", h.UpdateNode)
	huma.Delete(api, "/v1/nodes/{id}", h.DeleteNode)
	huma.Post(api, "/v1/nodes/batch-delete", h.BatchDeleteNodes)
}

func (h *NodeManagementHandler) ListNodes(ctx context.Context, input *ListNodesInput) (*ListNodesOutput, error) {
	limit := input.Limit
	if limit < 30 {
		limit = 30
	}
	if limit > 50 {
		limit = 50
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	groupID := strings.TrimSpace(input.GroupID)
	if groupID != "" {
		if _, err := h.groupRepo.FindByID(ctx, groupID); err != nil {
			if errors.Is(err, domain.ErrGroupNotFound) {
				return nil, huma.Error404NotFound("group not found")
			}
			h.logger.Error("failed to validate group for list nodes", slog.String("group_id", groupID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to list nodes")
		}
		if limit > 200 {
			limit = 200
		}
	}

	var nodes []domain.Node
	var err error
	if groupID != "" {
		nodes, err = h.nodeRepo.ListPageByGroup(ctx, groupID, limit+1, offset)
	} else {
		nodes, err = h.nodeRepo.ListPage(ctx, limit+1, offset)
	}
	if err != nil {
		h.logger.Error("failed to list nodes", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list nodes")
	}

	hasMore := len(nodes) > limit
	if hasMore {
		nodes = nodes[:limit]
	}

	response := h.buildNodeItems(ctx, nodes)

	out := &ListNodesOutput{}
	out.Body.Nodes = response
	out.Body.HasMore = hasMore
	if hasMore {
		nextOffset := offset + limit
		out.Body.NextOffset = &nextOffset
	}

	return out, nil
}

func (h *NodeManagementHandler) buildNodeItems(_ context.Context, nodes []domain.Node) []NodeItem {
	response := make([]NodeItem, 0, len(nodes))
	for _, n := range nodes {
		item := NodeItem{
			ID:        n.ID,
			URL:       n.URL,
			GroupIDs:  n.GroupIDs,
			Country:   domain.NormalizeCountryCode(n.Country),
			IsSelf:    n.IsSelf,
			ExpiresAt: formatOptionalExpiresAt(n.ExpiresAt),
		}
		if n.CountryInfo != nil {
			item.CountryCode = n.CountryInfo.CountryCode
			item.CountryName = n.CountryInfo.CountryName
			item.CountryFlag = n.CountryInfo.Flag
		}
		response = append(response, item)
	}

	return response
}

func (h *NodeManagementHandler) GetNode(ctx context.Context, input *GetNodeInput) (*GetNodeOutput, error) {
	node, err := h.nodeRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error404NotFound("node not found")
		}
		h.logger.Error("failed to get node", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to get node")
	}

	item := NodeItem{
		ID:        node.ID,
		URL:       node.URL,
		GroupIDs:  node.GroupIDs,
		Country:   domain.NormalizeCountryCode(node.Country),
		IsSelf:    node.IsSelf,
		ExpiresAt: formatOptionalExpiresAt(node.ExpiresAt),
	}
	if node.CountryInfo != nil {
		item.CountryCode = node.CountryInfo.CountryCode
		item.CountryName = node.CountryInfo.CountryName
		item.CountryFlag = node.CountryInfo.Flag
	}
	return &GetNodeOutput{Body: item}, nil
}

func (h *NodeManagementHandler) DeleteNode(ctx context.Context, input *DeleteNodeInput) (*struct{}, error) {
	if err := h.nodeRepo.Delete(ctx, input.ID); err != nil {
		h.logger.Error("failed to delete node", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete node")
	}

	if err := h.runtime.ForceSync(); err != nil {
		h.logger.Warn("failed to sync after node deletion", slog.String("id", input.ID), slog.String("error", err.Error()))
	}

	return nil, nil
}

type batchDeleteNodesInput struct {
	Body struct {
		IDs []string `json:"ids" required:"true"`
	}
}

func (h *NodeManagementHandler) BatchDeleteNodes(ctx context.Context, input *batchDeleteNodesInput) (*struct{}, error) {
	if len(input.Body.IDs) == 0 {
		return nil, huma.Error400BadRequest("ids are required")
	}
	for _, id := range input.Body.IDs {
		if err := h.nodeRepo.Delete(ctx, id); err != nil {
			h.logger.Error("failed to delete node in batch", slog.String("id", id), slog.String("error", err.Error()))
		}
	}

	if err := h.runtime.ForceSync(); err != nil {
		h.logger.Warn("failed to sync after batch node deletion", slog.String("error", err.Error()))
	}

	h.logger.Info("batch deleted nodes", slog.Int("count", len(input.Body.IDs)))
	return nil, nil
}

func generateNodeID(url string, groupIDs []string) string {
	hash := sha256.Sum256([]byte(url + "|" + strings.Join(groupIDs, ",")))
	return "node_" + hex.EncodeToString(hash[:8])
}

func parseOptionalExpiresAt(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, fmt.Errorf("parsing expires_at: %w", err)
	}
	t = t.UTC()
	return &t, nil
}

func formatOptionalExpiresAt(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
