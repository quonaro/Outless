package http

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
	"outless/internal/service"
)

type GroupManagementHandler struct {
	groupRepo           domain.GroupRepository
	topUpRepo           domain.GroupTopUpRepository
	nodeRepo            domain.NodeRepository
	subscriptionService *service.SubscriptionService
	topUpScheduler      *service.TopUpScheduler
	logger              *slog.Logger
}

func NewGroupManagementHandler(
	groupRepo domain.GroupRepository,
	topUpRepo domain.GroupTopUpRepository,
	nodeRepo domain.NodeRepository,
	subscriptionService *service.SubscriptionService,
	topUpScheduler *service.TopUpScheduler,
	logger *slog.Logger,
) *GroupManagementHandler {
	return &GroupManagementHandler{
		groupRepo:           groupRepo,
		topUpRepo:           topUpRepo,
		nodeRepo:            nodeRepo,
		subscriptionService: subscriptionService,
		topUpScheduler:      topUpScheduler,
		logger:              logger,
	}
}

type CreateGroupInput struct {
	Body struct {
		Name          string      `json:"name" required:"true" maxLength:"100"`
		RandomEnabled bool        `json:"random_enabled"`
		RandomLimit   *int        `json:"random_limit"`
		ShowOrigins   bool        `json:"show_origins"`
		TopUp         *TopUpInput `json:"top_up,omitempty"`
	}
}

type CreateGroupOutput struct {
	Body struct {
		ID            string       `json:"id"`
		Name          string       `json:"name"`
		RandomEnabled bool         `json:"random_enabled"`
		RandomLimit   *int         `json:"random_limit"`
		ShowOrigins   bool         `json:"show_origins"`
		IsTopUp       bool         `json:"is_topup"`
		TopUp         *TopUpOutput `json:"top_up,omitempty"`
		CreatedAt     time.Time    `json:"created_at"`
	}
}

type ListGroupsOutput struct {
	Body []GroupItem `json:"groups"`
}

type UpdateGroupInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		Name          string      `json:"name" required:"true" maxLength:"100"`
		RandomEnabled bool        `json:"random_enabled"`
		RandomLimit   *int        `json:"random_limit"`
		ShowOrigins   bool        `json:"show_origins"`
		TopUp         *TopUpInput `json:"top_up,omitempty"`
	}
}

type DeleteGroupInput struct {
	ID string `path:"id" required:"true"`
}

type GroupItem struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	TotalNodes    int       `json:"total_nodes"`
	RandomEnabled bool      `json:"random_enabled"`
	RandomLimit   *int      `json:"random_limit"`
	ShowOrigins   bool      `json:"show_origins"`
	IsTopUp       bool      `json:"is_topup"`
	TopUpID       string    `json:"top_up_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (h *GroupManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/groups", h.CreateGroup)
	huma.Get(api, "/v1/groups", h.ListGroups)
	huma.Put(api, "/v1/groups/{id}", h.UpdateGroup)
	huma.Delete(api, "/v1/groups/{id}", h.DeleteGroup)
}

func (h *GroupManagementHandler) CreateGroup(ctx context.Context, input *CreateGroupInput) (*CreateGroupOutput, error) {
	input.Body.Name = strings.TrimSpace(input.Body.Name)
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("name is required")
	}

	id, err := domain.GenerateGroupID()
	if err != nil {
		h.logger.Error("failed to generate group id", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create group")
	}

	group := domain.Group{
		ID:            id,
		Name:          input.Body.Name,
		RandomEnabled: input.Body.RandomEnabled,
		RandomLimit:   input.Body.RandomLimit,
		IsTopUp:       input.Body.TopUp != nil,
		ShowOrigins:   input.Body.ShowOrigins,
		CreatedAt:     time.Now().UTC(),
	}

	if !input.Body.RandomEnabled && input.Body.RandomLimit != nil {
		group.RandomEnabled = true
	}

	if err := h.groupRepo.Create(ctx, group); err != nil {
		h.logger.Error("failed to create group", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create group")
	}

	out := &CreateGroupOutput{}
	out.Body.ID = id
	out.Body.Name = group.Name
	out.Body.RandomEnabled = group.RandomEnabled
	out.Body.RandomLimit = group.RandomLimit
	out.Body.ShowOrigins = group.ShowOrigins
	out.Body.IsTopUp = group.IsTopUp
	out.Body.CreatedAt = group.CreatedAt

	if input.Body.TopUp != nil {
		topUp, err := parseTopUpInput(*input.Body.TopUp, group.ID)
		if err != nil {
			h.logger.Error("failed to parse top-up input", slog.String("error", err.Error()))
			return nil, huma.Error400BadRequest("invalid top-up configuration")
		}
		if err := h.topUpRepo.Create(ctx, topUp); err != nil {
			h.logger.Error("failed to create top-up", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to create top-up")
		}
		h.runTopUpNow(topUp.ID)
		out.Body.TopUp = topUpPtr(topUp)
	}

	return out, nil
}

func (h *GroupManagementHandler) ListGroups(ctx context.Context, _ *struct{}) (*ListGroupsOutput, error) {
	groups, err := h.groupRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list groups", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list groups")
	}

	topUpMap := make(map[string]string)
	if h.topUpRepo != nil {
		topUps, err := h.topUpRepo.List(ctx)
		if err != nil {
			h.logger.Error("failed to list top-ups", slog.String("error", err.Error()))
		} else {
			for _, t := range topUps {
				topUpMap[t.GroupID] = t.ID
			}
		}
	}

	response := make([]GroupItem, 0, len(groups))
	for _, g := range groups {
		topUpID := ""
		if g.IsTopUp {
			topUpID = topUpMap[g.ID]
		}
		response = append(response, GroupItem{
			ID:            g.ID,
			Name:          g.Name,
			TotalNodes:    g.TotalNodes,
			RandomEnabled: g.RandomEnabled,
			RandomLimit:   g.RandomLimit,
			ShowOrigins:   g.ShowOrigins,
			IsTopUp:       g.IsTopUp,
			TopUpID:       topUpID,
			CreatedAt:     g.CreatedAt,
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
		if errors.Is(err, domain.ErrGroupNotFound) {
			return nil, huma.Error404NotFound("group not found")
		}
		h.logger.Error("group not found", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find group")
	}

	group.Name = input.Body.Name
	group.RandomEnabled = input.Body.RandomEnabled
	group.RandomLimit = input.Body.RandomLimit
	group.ShowOrigins = input.Body.ShowOrigins
	if !group.RandomEnabled && group.RandomLimit != nil {
		group.RandomEnabled = true
	}

	if input.Body.TopUp != nil {
		group.IsTopUp = true
		if err := h.groupRepo.Update(ctx, group); err != nil {
			h.logger.Error("failed to update group", slog.String("id", input.ID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to update group")
		}
		if err := h.upsertTopUp(ctx, group.ID, *input.Body.TopUp); err != nil {
			h.logger.Error("failed to upsert top-up", slog.String("id", input.ID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to update top-up")
		}
	} else {
		if err := h.groupRepo.Update(ctx, group); err != nil {
			h.logger.Error("failed to update group", slog.String("id", input.ID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to update group")
		}
	}

	if h.subscriptionService != nil {
		h.subscriptionService.InvalidateGroupCache()
	}

	return nil, nil
}

func (h *GroupManagementHandler) upsertTopUp(ctx context.Context, groupID string, input TopUpInput) error {
	existing, err := h.topUpRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		if !errors.Is(err, domain.ErrGroupTopUpNotFound) {
			return err
		}
		topUp, err := parseTopUpInput(input, groupID)
		if err != nil {
			return err
		}
		if err := h.topUpRepo.Create(ctx, topUp); err != nil {
			return err
		}
		h.runTopUpNow(topUp.ID)
		return nil
	}

	merged, err := mergeTopUpInput(existing, input)
	if err != nil {
		return err
	}
	return h.topUpRepo.Update(ctx, merged)
}

func (h *GroupManagementHandler) runTopUpNow(id string) {
	if h.topUpScheduler == nil {
		return
	}
	h.topUpScheduler.RunAsync(id)
}

func (h *GroupManagementHandler) DeleteGroup(ctx context.Context, input *DeleteGroupInput) (*struct{}, error) {
	topUp, err := h.topUpRepo.FindByGroupID(ctx, input.ID)
	if err == nil {
		if err := h.topUpRepo.Delete(ctx, topUp.ID); err != nil {
			h.logger.Error("failed to delete top-up for group", slog.String("group_id", input.ID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to delete top-up")
		}
	}

	if err := h.groupRepo.Delete(ctx, input.ID); err != nil {
		h.logger.Error("failed to delete group", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete group")
	}
	if h.subscriptionService != nil {
		h.subscriptionService.InvalidateGroupCache()
	}

	return nil, nil
}
