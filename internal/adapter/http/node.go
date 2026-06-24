package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
)

type NodeManagementHandler struct {
	nodeRepo  domain.NodeRepository
	groupRepo domain.GroupRepository
	runtime   RuntimeController
	logger    *slog.Logger
}

func NewNodeManagementHandler(
	nodeRepo domain.NodeRepository,
	groupRepo domain.GroupRepository,
	runtime RuntimeController,
	logger *slog.Logger,
) *NodeManagementHandler {
	return &NodeManagementHandler{
		nodeRepo:  nodeRepo,
		groupRepo: groupRepo,
		runtime:   runtime,
		logger:    logger,
	}
}

type CreateNodeInput struct {
	Body struct {
		URL      string   `json:"url"`
		GroupIDs []string `json:"group_ids" required:"true"`
		IsSelf   bool     `json:"is_self"`
	}
}

type CreateNodeOutput struct {
	Body struct {
		ID       string   `json:"id"`
		URL      string   `json:"url"`
		GroupIDs []string `json:"group_ids"`
		IsSelf   bool     `json:"is_self"`
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
		URL      string   `json:"url,omitempty"`
		GroupIDs []string `json:"group_ids,omitempty"`
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
	ID       string   `json:"id"`
	URL      string   `json:"url"`
	GroupIDs []string `json:"group_ids"`
	Country  string   `json:"country"`
	IsSelf   bool     `json:"is_self"`
}

func (h *NodeManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/nodes", h.CreateNode)
	huma.Get(api, "/v1/nodes", h.ListNodes)
	huma.Get(api, "/v1/nodes/{id}", h.GetNode)
	huma.Patch(api, "/v1/nodes/{id}", h.UpdateNode)
	huma.Delete(api, "/v1/nodes/{id}", h.DeleteNode)
	huma.Post(api, "/v1/nodes/batch-delete", h.BatchDeleteNodes)
}

func (h *NodeManagementHandler) CreateNode(ctx context.Context, input *CreateNodeInput) (*CreateNodeOutput, error) {
	if !input.Body.IsSelf && input.Body.URL == "" {
		return nil, huma.Error400BadRequest("url is required when is_self is false")
	}

	if len(input.Body.GroupIDs) == 0 {
		return nil, huma.Error400BadRequest("group_ids is required")
	}

	for _, groupID := range input.Body.GroupIDs {
		if _, err := h.groupRepo.FindByID(ctx, groupID); err != nil {
			if errors.Is(err, domain.ErrGroupNotFound) {
				h.logger.Warn("group not found", slog.String("group_id", groupID))
				return nil, huma.Error400BadRequest("group not found")
			}
			h.logger.Error("failed to find group", slog.String("group_id", groupID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to validate group")
		}
	}

	if input.Body.IsSelf {
		exists, err := h.nodeRepo.HasSelfNode(ctx)
		if err != nil {
			h.logger.Error("failed to check self node", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to validate self node")
		}
		if exists {
			return nil, huma.Error409Conflict("self node already exists")
		}
	}

	nodeID := generateNodeID(input.Body.URL, input.Body.GroupIDs)
	if input.Body.IsSelf {
		nodeID = "self_" + strings.Join(input.Body.GroupIDs, "_")
	}

	node := domain.Node{
		ID:       nodeID,
		URL:      input.Body.URL,
		GroupIDs: input.Body.GroupIDs,
		IsSelf:   input.Body.IsSelf,
	}

	if err := h.nodeRepo.Create(ctx, node); err != nil {
		if errors.Is(err, domain.ErrDuplicateNode) {
			return nil, huma.Error409Conflict("node already exists")
		}
		h.logger.Error("failed to create node", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create node")
	}

	if err := h.runtime.ForceSync(); err != nil {
		h.logger.Warn("failed to sync after node creation", slog.String("error", err.Error()))
	}

	out := &CreateNodeOutput{}
	out.Body.ID = nodeID
	out.Body.URL = input.Body.URL
	out.Body.GroupIDs = input.Body.GroupIDs
	out.Body.IsSelf = input.Body.IsSelf

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

	response, hasMore := h.buildNodeItems(ctx, nodes, limit, offset)

	out := &ListNodesOutput{}
	out.Body.Nodes = response
	out.Body.HasMore = hasMore
	if hasMore {
		nextOffset := offset + limit
		out.Body.NextOffset = &nextOffset
	}

	return out, nil
}

func (h *NodeManagementHandler) buildNodeItems(
	_ context.Context,
	nodes []domain.Node,
	limit int,
	_ int,
) ([]NodeItem, bool) {
	response := make([]NodeItem, 0, len(nodes))
	for _, n := range nodes {
		response = append(response, NodeItem{
			ID:       n.ID,
			URL:      n.URL,
			GroupIDs: n.GroupIDs,
			Country:  domain.NormalizeCountryCode(n.Country),
			IsSelf:   n.IsSelf,
		})
	}

	if len(response) > limit {
		response = response[:limit]
		return response, true
	}
	return response, false
}

func (h *NodeManagementHandler) UpdateNode(ctx context.Context, input *UpdateNodeInput) (*struct{}, error) {
	if input.Body.URL == "" && len(input.Body.GroupIDs) == 0 {
		return nil, huma.Error400BadRequest("at least one field (url or group_ids) is required")
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
		ID:       input.ID,
		URL:      existingNode.URL,
		GroupIDs: existingNode.GroupIDs,
	}

	if input.Body.URL != "" {
		updates.URL = input.Body.URL
	}

	if len(input.Body.GroupIDs) > 0 {
		for _, groupID := range input.Body.GroupIDs {
			if _, err := h.groupRepo.FindByID(ctx, groupID); err != nil {
				if errors.Is(err, domain.ErrGroupNotFound) {
					h.logger.Warn("group not found", slog.String("group_id", groupID))
					return nil, huma.Error400BadRequest("group not found")
				}
				h.logger.Error("failed to find group", slog.String("group_id", groupID), slog.String("error", err.Error()))
				return nil, huma.Error500InternalServerError("failed to validate group")
			}
		}
		updates.GroupIDs = input.Body.GroupIDs
	}

	if err := h.nodeRepo.Update(ctx, updates); err != nil {
		h.logger.Error("failed to update node", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update node")
	}

	if err := h.runtime.ForceSync(); err != nil {
		h.logger.Warn("failed to sync after node update", slog.String("id", input.ID), slog.String("error", err.Error()))
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
			ID:       node.ID,
			URL:      node.URL,
			GroupIDs: node.GroupIDs,
			Country:  domain.NormalizeCountryCode(node.Country),
			IsSelf:   node.IsSelf,
		},
	}, nil
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
