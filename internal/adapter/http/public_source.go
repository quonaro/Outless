package http

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"outless/internal/service"
	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
)

type PublicSourceManagementHandler struct {
	sourceRepo domain.PublicSourceRepository
	groupRepo  domain.GroupRepository
	publicSvc  *service.PublicService
	logger     *slog.Logger
}

func NewPublicSourceManagementHandler(
	sourceRepo domain.PublicSourceRepository,
	groupRepo domain.GroupRepository,
	publicSvc *service.PublicService,
	logger *slog.Logger,
) *PublicSourceManagementHandler {
	return &PublicSourceManagementHandler{
		sourceRepo: sourceRepo,
		groupRepo:  groupRepo,
		publicSvc:  publicSvc,
		logger:     logger,
	}
}

type CreatePublicSourceInput struct {
	Body struct {
		URL     string `json:"url" required:"true"`
		GroupID string `json:"group_id" required:"true"`
	}
}

type CreatePublicSourceOutput struct {
	Body struct {
		ID        string    `json:"id"`
		URL       string    `json:"url"`
		GroupID   string    `json:"group_id"`
		CreatedAt time.Time `json:"created_at"`
	}
}

type ListPublicSourcesOutput struct {
	Body []PublicSourceItem `json:"public_sources"`
}

type UpdatePublicSourceInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		URL     string `json:"url" required:"true"`
		GroupID string `json:"group_id" required:"true"`
	}
}

type DeletePublicSourceInput struct {
	ID string `path:"id" required:"true"`
}

type PublicSourceItem struct {
	ID            string     `json:"id"`
	URL           string     `json:"url"`
	GroupID       string     `json:"group_id"`
	LastFetchedAt *time.Time `json:"last_fetched_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type SyncPublicSourceInput struct {
	ID string `path:"id" required:"true"`
}

func (h *PublicSourceManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/public-sources", h.CreatePublicSource)
	huma.Get(api, "/v1/public-sources", h.ListPublicSources)
	huma.Put(api, "/v1/public-sources/{id}", h.UpdatePublicSource)
	huma.Delete(api, "/v1/public-sources/{id}", h.DeletePublicSource)
	huma.Post(api, "/v1/public-sources/{id}/sync", h.SyncPublicSource)
}

func (h *PublicSourceManagementHandler) CreatePublicSource(ctx context.Context, input *CreatePublicSourceInput) (*CreatePublicSourceOutput, error) {
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

	id, err := domain.GeneratePublicSourceID()
	if err != nil {
		h.logger.Error("failed to generate source id", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create public source")
	}

	source := domain.PublicSource{
		ID:        id,
		URL:       input.Body.URL,
		GroupID:   input.Body.GroupID,
		CreatedAt: time.Now().UTC(),
	}

	if err := h.sourceRepo.Create(ctx, source); err != nil {
		h.logger.Error("failed to create public source", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create public source")
	}

	out := &CreatePublicSourceOutput{}
	out.Body.ID = id
	out.Body.URL = input.Body.URL
	out.Body.GroupID = input.Body.GroupID
	out.Body.CreatedAt = source.CreatedAt

	return out, nil
}

func (h *PublicSourceManagementHandler) ListPublicSources(ctx context.Context, _ *struct{}) (*ListPublicSourcesOutput, error) {
	sources, err := h.sourceRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list public sources", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list public sources")
	}

	response := make([]PublicSourceItem, 0, len(sources))

	for _, s := range sources {
		response = append(response, PublicSourceItem{
			ID:            s.ID,
			URL:           s.URL,
			GroupID:       s.GroupID,
			LastFetchedAt: s.LastFetchedAt,
			CreatedAt:     s.CreatedAt,
		})
	}

	out := &ListPublicSourcesOutput{}
	out.Body = response

	return out, nil
}

func (h *PublicSourceManagementHandler) UpdatePublicSource(ctx context.Context, input *UpdatePublicSourceInput) (*struct{}, error) {
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

	source, err := h.sourceRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error404NotFound("public source not found")
		}
		h.logger.Error("public source not found", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find public source")
	}

	source.URL = input.Body.URL
	source.GroupID = input.Body.GroupID

	if err := h.sourceRepo.Update(ctx, source); err != nil {
		h.logger.Error("failed to update public source", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update public source")
	}

	return nil, nil
}

func (h *PublicSourceManagementHandler) DeletePublicSource(ctx context.Context, input *DeletePublicSourceInput) (*struct{}, error) {
	if err := h.sourceRepo.Delete(ctx, input.ID); err != nil {
		h.logger.Error("failed to delete public source", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete public source")
	}

	return nil, nil
}

func (h *PublicSourceManagementHandler) SyncPublicSource(ctx context.Context, input *SyncPublicSourceInput) (*struct{}, error) {
	if err := h.publicSvc.ImportNodes(ctx, input.ID); err != nil {
		h.logger.Error("failed to sync public source", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to sync public source")
	}

	return nil, nil
}
