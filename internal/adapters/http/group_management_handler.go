package httpadapter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"outless/internal/adapters/postgres"
	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
)

type GroupManagementHandler struct {
	groupRepo domain.GroupRepository
	nodeRepo  domain.NodeRepository
	jobRepo   domain.ProbeJobRepository
	realtime  *RealtimeHandler
	logger    *slog.Logger
}

func NewGroupManagementHandler(groupRepo domain.GroupRepository, nodeRepo domain.NodeRepository, jobRepo domain.ProbeJobRepository, realtime *RealtimeHandler, logger *slog.Logger) *GroupManagementHandler {
	return &GroupManagementHandler{
		groupRepo: groupRepo,
		nodeRepo:  nodeRepo,
		jobRepo:   jobRepo,
		realtime:  realtime,
		logger:    logger,
	}
}

type CreateGroupInput struct {
	Body struct {
		Name                  string `json:"name" required:"true" maxLength:"100"`
		SourceURL             string `json:"source_url"`
		AutoDeleteUnavailable bool   `json:"auto_delete_unavailable"`
	}
}

type CreateGroupOutput struct {
	Body struct {
		ID                    string     `json:"id"`
		Name                  string     `json:"name"`
		SourceURL             string     `json:"source_url"`
		AutoDeleteUnavailable bool       `json:"auto_delete_unavailable"`
		LastSyncedAt          *time.Time `json:"last_synced_at"`
		CreatedAt             time.Time  `json:"created_at"`
	}
}

type ListGroupsOutput struct {
	Body []GroupItem `json:"groups"`
}

type UpdateGroupInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		Name                  string `json:"name" required:"true" maxLength:"100"`
		SourceURL             string `json:"source_url"`
		AutoDeleteUnavailable bool   `json:"auto_delete_unavailable"`
	}
}

type DeleteGroupInput struct {
	ID string `path:"id" required:"true"`
}

type DeleteUnavailableNodesInput struct {
	ID string `path:"id" required:"true"`
}

type DeleteUnavailableNodesOutput struct {
	Body struct {
		Deleted int64 `json:"deleted"`
	}
}

type ProbeUnavailableNodesInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		Mode     string `json:"mode,omitempty"`
		ProbeURL string `json:"probe_url,omitempty"`
	}
}

type ProbeUnavailableNodesOutput struct {
	Status int `json:"-" status:"202"`
	Body   struct {
		BatchID  string `json:"batch_id"`
		Enqueued int    `json:"enqueued"`
		Status   string `json:"status"`
	}
}

type GetGroupProbeUnavailableStateInput struct {
	ID string `path:"id" required:"true"`
}

type GetGroupProbeUnavailableStateOutput struct {
	Body GroupProbeUnavailableState `json:"state"`
}

type GroupItem struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	SourceURL             string     `json:"source_url"`
	TotalNodes            int        `json:"total_nodes"`
	HealthyNodes          int        `json:"healthy_nodes"`
	UnhealthyNodes        int        `json:"unhealthy_nodes"`
	UnknownNodes          int        `json:"unknown_nodes"`
	AutoDeleteUnavailable bool       `json:"auto_delete_unavailable"`
	LastSyncedAt          *time.Time `json:"last_synced_at"`
	CreatedAt             time.Time  `json:"created_at"`
}

func (h *GroupManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/groups", h.CreateGroup)
	huma.Get(api, "/v1/groups", h.ListGroups)
	huma.Put(api, "/v1/groups/{id}", h.UpdateGroup)
	huma.Post(api, "/v1/groups/{id}/nodes/delete-unavailable", h.DeleteUnavailableNodes)
	huma.Post(api, "/v1/groups/{id}/nodes/probe-unavailable", h.ProbeUnavailableNodes)
	huma.Get(api, "/v1/groups/{id}/probe-unavailable-state", h.GetGroupProbeUnavailableState)
	huma.Delete(api, "/v1/groups/{id}", h.DeleteGroup)
}

func (h *GroupManagementHandler) CreateGroup(ctx context.Context, input *CreateGroupInput) (*CreateGroupOutput, error) {
	input.Body.Name = strings.TrimSpace(input.Body.Name)
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("name is required")
	}

	id, err := postgres.GenerateGroupID()
	if err != nil {
		h.logger.Error("failed to generate group id", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create group")
	}

	group := domain.Group{
		ID:                    id,
		Name:                  input.Body.Name,
		SourceURL:             strings.TrimSpace(input.Body.SourceURL),
		AutoDeleteUnavailable: input.Body.AutoDeleteUnavailable,
		CreatedAt:             time.Now().UTC(),
	}

	if err := h.groupRepo.Create(ctx, group); err != nil {
		h.logger.Error("failed to create group", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create group")
	}
	if h.realtime != nil {
		h.realtime.NotifyInvalidate(false, true)
	}

	out := &CreateGroupOutput{}
	out.Body.ID = id
	out.Body.Name = group.Name
	out.Body.SourceURL = group.SourceURL
	out.Body.AutoDeleteUnavailable = group.AutoDeleteUnavailable
	out.Body.LastSyncedAt = group.LastSyncedAt
	out.Body.CreatedAt = group.CreatedAt

	return out, nil
}

func (h *GroupManagementHandler) ListGroups(ctx context.Context, _ *struct{}) (*ListGroupsOutput, error) {
	groups, err := h.groupRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list groups", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list groups")
	}

	response := make([]GroupItem, 0, len(groups))

	for _, g := range groups {
		response = append(response, GroupItem{
			ID:                    g.ID,
			Name:                  g.Name,
			SourceURL:             g.SourceURL,
			TotalNodes:            g.TotalNodes,
			HealthyNodes:          g.HealthyNodes,
			UnhealthyNodes:        g.UnhealthyNodes,
			UnknownNodes:          g.UnknownNodes,
			AutoDeleteUnavailable: g.AutoDeleteUnavailable,
			LastSyncedAt:          g.LastSyncedAt,
			CreatedAt:             g.CreatedAt,
		})
	}

	out := &ListGroupsOutput{}
	out.Body = response

	return out, nil
}

func (h *GroupManagementHandler) UpdateGroup(ctx context.Context, input *UpdateGroupInput) (*struct{}, error) {
	input.Body.Name = strings.TrimSpace(input.Body.Name)
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("name is required")
	}

	group, err := h.groupRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error404NotFound("group not found")
		}
		h.logger.Error("group not found", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find group")
	}

	group.Name = input.Body.Name
	group.SourceURL = strings.TrimSpace(input.Body.SourceURL)
	group.AutoDeleteUnavailable = input.Body.AutoDeleteUnavailable
	if err := h.groupRepo.Update(ctx, group); err != nil {
		h.logger.Error("failed to update group", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update group")
	}
	if h.realtime != nil {
		h.realtime.NotifyInvalidate(false, true)
	}

	return nil, nil
}

func (h *GroupManagementHandler) DeleteUnavailableNodes(ctx context.Context, input *DeleteUnavailableNodesInput) (*DeleteUnavailableNodesOutput, error) {
	if _, err := h.groupRepo.FindByID(ctx, input.ID); err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error404NotFound("group not found")
		}
		h.logger.Error("failed to find group", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find group")
	}

	deleted, err := h.nodeRepo.DeleteUnavailableByGroup(ctx, input.ID)
	if err != nil {
		h.logger.Error("failed to delete unavailable nodes", slog.String("group_id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete unavailable nodes")
	}

	out := &DeleteUnavailableNodesOutput{}
	out.Body.Deleted = deleted
	if h.realtime != nil {
		h.realtime.NotifyInvalidate(true, true)
	}
	return out, nil
}

func (h *GroupManagementHandler) GetGroupProbeUnavailableState(ctx context.Context, input *GetGroupProbeUnavailableStateInput) (*GetGroupProbeUnavailableStateOutput, error) {
	if _, err := h.groupRepo.FindByID(ctx, input.ID); err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error404NotFound("group not found")
		}
		h.logger.Error("failed to find group", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find group")
	}
	if h.realtime == nil {
		return &GetGroupProbeUnavailableStateOutput{Body: idleGroupProbeUnavailableState()}, nil
	}
	return &GetGroupProbeUnavailableStateOutput{Body: h.realtime.ProbeUnavailableStateForGroup(input.ID)}, nil
}

func (h *GroupManagementHandler) ProbeUnavailableNodes(ctx context.Context, input *ProbeUnavailableNodesInput) (*ProbeUnavailableNodesOutput, error) {
	if _, err := h.groupRepo.FindByID(ctx, input.ID); err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error404NotFound("group not found")
		}
		h.logger.Error("failed to find group", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find group")
	}

	nodes, err := h.nodeRepo.ListNonHealthyByGroup(ctx, input.ID)
	if err != nil {
		h.logger.Error("failed to list non-healthy nodes", slog.String("group_id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list nodes")
	}

	batchID := newProbeBatchID()
	requestedBy := requestedByFromContext(ctx)
	mode := parseProbeMode(input.Body.Mode)
	jobs := make([]domain.ProbeJobCreate, 0, len(nodes))
	for _, node := range nodes {
		jobs = append(jobs, domain.ProbeJobCreate{
			BatchID:     batchID,
			NodeID:      node.ID,
			GroupID:     node.GroupID,
			RequestedBy: requestedBy,
			Mode:        mode,
			ProbeURL:    input.Body.ProbeURL,
		})
	}
	if _, err := h.jobRepo.EnqueueBatch(ctx, jobs); err != nil {
		h.logger.Error("failed to enqueue probe jobs batch", slog.String("group_id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to enqueue probe jobs")
	}

	out := &ProbeUnavailableNodesOutput{Status: 202}
	out.Body.BatchID = batchID
	out.Body.Enqueued = len(jobs)
	out.Body.Status = string(domain.ProbeJobStatusPending)
	return out, nil
}

func (h *GroupManagementHandler) DeleteGroup(ctx context.Context, input *DeleteGroupInput) (*struct{}, error) {
	if err := h.groupRepo.Delete(ctx, input.ID); err != nil {
		h.logger.Error("failed to delete group", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete group")
	}
	if h.realtime != nil {
		h.realtime.NotifyInvalidate(true, true)
	}

	return nil, nil
}

func newProbeBatchID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("probe_batch_%d", time.Now().UTC().UnixNano())
	}
	return "probe_batch_" + hex.EncodeToString(b[:])
}
