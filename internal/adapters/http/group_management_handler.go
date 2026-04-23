package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"outless/internal/adapters/postgres"
	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
)

type GroupManagementHandler struct {
	groupRepo domain.GroupRepository
	logger    *slog.Logger
}

func NewGroupManagementHandler(groupRepo domain.GroupRepository, logger *slog.Logger) *GroupManagementHandler {
	return &GroupManagementHandler{
		groupRepo: groupRepo,
		logger:    logger,
	}
}

type CreateGroupInput struct {
	Body struct {
		Name string `json:"name" required:"true" maxLength:"100"`
	}
}

type CreateGroupOutput struct {
	Body struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"created_at"`
	}
}

type ListGroupsOutput struct {
	Body []GroupItem `json:"groups"`
}

type UpdateGroupInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		Name string `json:"name" required:"true" maxLength:"100"`
	}
}

type DeleteGroupInput struct {
	ID string `path:"id" required:"true"`
}

type GroupItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *GroupManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/groups", h.CreateGroup)
	huma.Get(api, "/v1/groups", h.ListGroups)
	huma.Put(api, "/v1/groups/{id}", h.UpdateGroup)
	huma.Delete(api, "/v1/groups/{id}", h.DeleteGroup)
}

func (h *GroupManagementHandler) CreateGroup(ctx context.Context, input *CreateGroupInput) (*CreateGroupOutput, error) {
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("name is required")
	}

	id, err := postgres.GenerateGroupID()
	if err != nil {
		h.logger.Error("failed to generate group id", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create group")
	}

	group := domain.Group{
		ID:        id,
		Name:      input.Body.Name,
		CreatedAt: time.Now().UTC(),
	}

	if err := h.groupRepo.Create(ctx, group); err != nil {
		h.logger.Error("failed to create group", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create group")
	}

	out := &CreateGroupOutput{}
	out.Body.ID = id
	out.Body.Name = input.Body.Name
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
			ID:        g.ID,
			Name:      g.Name,
			CreatedAt: g.CreatedAt,
		})
	}

	out := &ListGroupsOutput{}
	out.Body = response

	return out, nil
}

func (h *GroupManagementHandler) UpdateGroup(ctx context.Context, input *UpdateGroupInput) (*struct{}, error) {
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
	if err := h.groupRepo.Update(ctx, group); err != nil {
		h.logger.Error("failed to update group", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update group")
	}

	return nil, nil
}

func (h *GroupManagementHandler) DeleteGroup(ctx context.Context, input *DeleteGroupInput) (*struct{}, error) {
	if err := h.groupRepo.Delete(ctx, input.ID); err != nil {
		h.logger.Error("failed to delete group", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete group")
	}

	return nil, nil
}
