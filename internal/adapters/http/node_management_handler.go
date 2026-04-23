package httpadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"

	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
)

type NodeManagementHandler struct {
	nodeRepo  domain.NodeRepository
	groupRepo domain.GroupRepository
	logger    *slog.Logger
}

func NewNodeManagementHandler(nodeRepo domain.NodeRepository, groupRepo domain.GroupRepository, logger *slog.Logger) *NodeManagementHandler {
	return &NodeManagementHandler{
		nodeRepo:  nodeRepo,
		groupRepo: groupRepo,
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
		Status  string `json:"status"`
	}
}

type ListNodesOutput struct {
	Body []NodeItem `json:"nodes"`
}

type UpdateNodeInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		URL     string `json:"url" required:"true"`
		GroupID string `json:"group_id"`
	}
}

type DeleteNodeInput struct {
	ID string `path:"id" required:"true"`
}

type NodeItem struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	GroupID string `json:"group_id"`
	Latency int64  `json:"latency_ms"`
	Status  string `json:"status"`
	Country string `json:"country"`
}

func (h *NodeManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/nodes", h.CreateNode)
	huma.Get(api, "/v1/nodes", h.ListNodes)
	huma.Put(api, "/v1/nodes/{id}", h.UpdateNode)
	huma.Delete(api, "/v1/nodes/{id}", h.DeleteNode)
}

func (h *NodeManagementHandler) CreateNode(ctx context.Context, input *CreateNodeInput) (*CreateNodeOutput, error) {
	if input.Body.URL == "" {
		return nil, huma.Error400BadRequest("url is required")
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
	}

	nodeID := generateNodeID(input.Body.URL)
	node := domain.Node{
		ID:      nodeID,
		URL:     input.Body.URL,
		GroupID: input.Body.GroupID,
		Status:  domain.NodeStatusUnknown,
	}

	if err := h.nodeRepo.Create(ctx, node); err != nil {
		h.logger.Error("failed to create node", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create node")
	}

	out := &CreateNodeOutput{}
	out.Body.ID = nodeID
	out.Body.URL = input.Body.URL
	out.Body.GroupID = input.Body.GroupID
	out.Body.Status = string(domain.NodeStatusUnknown)

	return out, nil
}

func (h *NodeManagementHandler) ListNodes(ctx context.Context, _ *struct{}) (*ListNodesOutput, error) {
	nodes, err := h.nodeRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list nodes", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list nodes")
	}

	response := make([]NodeItem, 0, len(nodes))

	for _, n := range nodes {
		response = append(response, NodeItem{
			ID:      n.ID,
			URL:     n.URL,
			GroupID: n.GroupID,
			Latency: int64(n.Latency.Milliseconds()),
			Status:  string(n.Status),
			Country: n.Country,
		})
	}

	out := &ListNodesOutput{}
	out.Body = response

	return out, nil
}

func (h *NodeManagementHandler) UpdateNode(ctx context.Context, input *UpdateNodeInput) (*struct{}, error) {
	if input.Body.URL == "" {
		return nil, huma.Error400BadRequest("url is required")
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
	}

	node := domain.Node{
		ID:      input.ID,
		URL:     input.Body.URL,
		GroupID: input.Body.GroupID,
	}

	if err := h.nodeRepo.Update(ctx, node); err != nil {
		h.logger.Error("failed to update node", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update node")
	}

	return nil, nil
}

func (h *NodeManagementHandler) DeleteNode(ctx context.Context, input *DeleteNodeInput) (*struct{}, error) {
	if err := h.nodeRepo.Delete(ctx, input.ID); err != nil {
		h.logger.Error("failed to delete node", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete node")
	}

	return nil, nil
}

func generateNodeID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return "node_" + hex.EncodeToString(hash[:8])
}
