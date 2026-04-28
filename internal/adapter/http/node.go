package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"

	"outless/internal/domain"
	"outless/shared/vless"

	"github.com/danielgtaylor/huma/v2"
)

type NodeManagementHandler struct {
	nodeRepo  domain.NodeRepository
	groupRepo domain.GroupRepository
	geoip     domain.GeoIPResolver
	realtime  *RealtimeHandler
	logger    *slog.Logger
}

func NewNodeManagementHandler(nodeRepo domain.NodeRepository, groupRepo domain.GroupRepository, geoip domain.GeoIPResolver, realtime *RealtimeHandler, logger *slog.Logger) *NodeManagementHandler {
	return &NodeManagementHandler{
		nodeRepo:  nodeRepo,
		groupRepo: groupRepo,
		geoip:     geoip,
		realtime:  realtime,
		logger:    logger,
	}
}

type CreateNodeInput struct {
	Body struct {
		URL     string `json:"url" required:"true"`
		GroupID string `json:"group_id"`
	}
}

type CreateNodeOutput struct {
	Body struct {
		ID      string `json:"id"`
		URL     string `json:"url"`
		GroupID string `json:"group_id"`
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

type UpdateNodeInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		URL     string `json:"url,omitempty"`
		GroupID string `json:"group_id,omitempty"`
	}
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
	ID      string `json:"id"`
	URL     string `json:"url"`
	GroupID string `json:"group_id"`
	Country string `json:"country"`
}

func (h *NodeManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/nodes", h.CreateNode)
	huma.Get(api, "/v1/nodes", h.ListNodes)
	huma.Get(api, "/v1/nodes/{id}", h.GetNode)
	huma.Patch(api, "/v1/nodes/{id}", h.UpdateNode)
	huma.Delete(api, "/v1/nodes/{id}", h.DeleteNode)
}

func (h *NodeManagementHandler) CreateNode(ctx context.Context, input *CreateNodeInput) (*CreateNodeOutput, error) {
	if input.Body.URL == "" {
		return nil, huma.Error400BadRequest("url is required")
	}

	if input.Body.GroupID == "" {
		return nil, huma.Error400BadRequest("group_id is required")
	}

	if _, err := h.groupRepo.FindByID(ctx, input.Body.GroupID); err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			h.logger.Warn("group not found", slog.String("group_id", input.Body.GroupID))
			return nil, huma.Error400BadRequest("group not found")
		}
		h.logger.Error("failed to find group", slog.String("group_id", input.Body.GroupID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to validate group")
	}

	nodeID := generateNodeID(input.Body.URL, input.Body.GroupID)

	// Resolve country from IP address using GeoIP
	country := ""
	if h.geoip != nil {
		ip := vless.ExtractIPFromVLESS(input.Body.URL)
		if ip != "" {
			if resolvedCountry, err := h.geoip.LookupCountry(ctx, ip); err == nil {
				country = domain.NormalizeCountryCode(resolvedCountry)
			} else {
				h.logger.Debug("geoip lookup failed", slog.String("ip", ip), slog.String("error", err.Error()))
			}
		}
	}

	node := domain.Node{
		ID:      nodeID,
		URL:     input.Body.URL,
		GroupID: input.Body.GroupID,
		Country: country,
	}

	if err := h.nodeRepo.Create(ctx, node); err != nil {
		if errors.Is(err, domain.ErrDuplicateNode) {
			return nil, huma.Error409Conflict("node already exists")
		}
		h.logger.Error("failed to create node", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create node")
	}
	if h.realtime != nil {
		h.realtime.NotifyInvalidate(true, true)
	}

	out := &CreateNodeOutput{}
	out.Body.ID = nodeID
	out.Body.URL = input.Body.URL
	out.Body.GroupID = input.Body.GroupID

	return out, nil
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
			// Group repo wraps gorm not-found with domain.ErrNodeNotFound today.
			if errors.Is(err, domain.ErrNodeNotFound) {
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

	response := make([]NodeItem, 0, len(nodes))

	for _, n := range nodes {
		response = append(response, NodeItem{
			ID:      n.ID,
			URL:     n.URL,
			GroupID: n.GroupID,
			Country: domain.NormalizeCountryCode(n.Country),
		})
	}

	out := &ListNodesOutput{}
	out.Body.Nodes = response
	out.Body.HasMore = hasMore
	if hasMore {
		nextOffset := offset + limit
		out.Body.NextOffset = &nextOffset
	}

	return out, nil
}

func (h *NodeManagementHandler) UpdateNode(ctx context.Context, input *UpdateNodeInput) (*struct{}, error) {
	if input.Body.URL == "" && input.Body.GroupID == "" {
		return nil, huma.Error400BadRequest("at least one field (url or group_id) is required")
	}

	existingNode, err := h.nodeRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error404NotFound("node not found")
		}
		h.logger.Error("failed to find node for update", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find node")
	}

	updates := domain.Node{
		ID:      input.ID,
		URL:     existingNode.URL,
		GroupID: existingNode.GroupID,
	}

	if input.Body.URL != "" {
		updates.URL = input.Body.URL
	}

	if input.Body.GroupID != "" {
		if _, err := h.groupRepo.FindByID(ctx, input.Body.GroupID); err != nil {
			if errors.Is(err, domain.ErrNodeNotFound) {
				h.logger.Warn("group not found", slog.String("group_id", input.Body.GroupID))
				return nil, huma.Error400BadRequest("group not found")
			}
			h.logger.Error("failed to find group", slog.String("group_id", input.Body.GroupID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to validate group")
		}
		updates.GroupID = input.Body.GroupID
	}

	if err := h.nodeRepo.Update(ctx, updates); err != nil {
		h.logger.Error("failed to update node", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update node")
	}
	if h.realtime != nil {
		h.realtime.NotifyInvalidate(true, true)
	}

	return nil, nil
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

	return &GetNodeOutput{
		Body: NodeItem{
			ID:      node.ID,
			URL:     node.URL,
			GroupID: node.GroupID,
			Country: domain.NormalizeCountryCode(node.Country),
		},
	}, nil
}

func (h *NodeManagementHandler) DeleteNode(ctx context.Context, input *DeleteNodeInput) (*struct{}, error) {
	if err := h.nodeRepo.Delete(ctx, input.ID); err != nil {
		h.logger.Error("failed to delete node", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete node")
	}
	if h.realtime != nil {
		h.realtime.NotifyInvalidate(true, true)
	}

	return nil, nil
}

func generateNodeID(url, groupID string) string {
	hash := sha256.Sum256([]byte(url + "|" + groupID))
	return "node_" + hex.EncodeToString(hash[:8])
}
