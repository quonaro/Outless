package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
)

type TokenManagementHandler struct {
	tokenRepo domain.TokenRepository
	groupRepo domain.GroupRepository
	logger    *slog.Logger
}

func NewTokenManagementHandler(tokenRepo domain.TokenRepository, groupRepo domain.GroupRepository, logger *slog.Logger) *TokenManagementHandler {
	return &TokenManagementHandler{
		tokenRepo: tokenRepo,
		groupRepo: groupRepo,
		logger:    logger,
	}
}

type CreateTokenInput struct {
	Body struct {
		Owner     string `json:"owner" required:"true" maxLength:"64"`
		GroupID   string `json:"group_id" required:"true"`
		ExpiresIn string `json:"expires_in" example:"24h"`
	}
}

type CreateTokenOutput struct {
	Body struct {
		ID        string    `json:"id"`
		Token     string    `json:"token"`
		Owner     string    `json:"owner"`
		GroupID   string    `json:"group_id"`
		IsActive  bool      `json:"is_active"`
		ExpiresAt time.Time `json:"expires_at"`
		CreatedAt time.Time `json:"created_at"`
	}
}

type ListTokensOutput struct {
	Body []TokenItem `json:"tokens"`
}

type DeleteTokenInput struct {
	ID string `path:"id" required:"true"`
}

type TokenItem struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	GroupID   string    `json:"group_id"`
	IsActive  bool      `json:"is_active"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *TokenManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/tokens", h.CreateToken)
	huma.Get(api, "/v1/tokens", h.ListTokens)
	huma.Delete(api, "/v1/tokens/{id}", h.DeleteToken)
}

func (h *TokenManagementHandler) CreateToken(ctx context.Context, input *CreateTokenInput) (*CreateTokenOutput, error) {
	if input.Body.Owner == "" {
		return nil, huma.Error400BadRequest("owner is required")
	}

	if input.Body.GroupID == "" {
		return nil, huma.Error400BadRequest("group_id is required")
	}

	// Validate group exists
	if _, err := h.groupRepo.FindByID(ctx, input.Body.GroupID); err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			h.logger.Warn("group not found", slog.String("group_id", input.Body.GroupID))
			return nil, huma.Error400BadRequest("group not found")
		}
		h.logger.Error("failed to find group", slog.String("group_id", input.Body.GroupID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to validate group")
	}

	// Parse expiration
	expiresIn := 30 * 24 * time.Hour // default 30 days
	if input.Body.ExpiresIn != "" {
		d, err := time.ParseDuration(input.Body.ExpiresIn)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid expires_in format")
		}
		expiresIn = d
	}

	expiresAt := time.Now().UTC().Add(expiresIn)
	token, err := h.tokenRepo.IssueToken(ctx, input.Body.Owner, input.Body.GroupID, expiresAt)
	if err != nil {
		h.logger.Error("failed to issue token", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create token")
	}

	out := &CreateTokenOutput{}
	out.Body.ID = token
	out.Body.Token = token
	out.Body.Owner = input.Body.Owner
	out.Body.GroupID = input.Body.GroupID
	out.Body.IsActive = true
	out.Body.ExpiresAt = expiresAt
	out.Body.CreatedAt = time.Now().UTC()

	return out, nil
}

func (h *TokenManagementHandler) ListTokens(ctx context.Context, _ *struct{}) (*ListTokensOutput, error) {
	tokens, err := h.tokenRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list tokens", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list tokens")
	}

	response := make([]TokenItem, 0, len(tokens))

	for _, t := range tokens {
		response = append(response, TokenItem{
			ID:        t.ID,
			Owner:     t.Owner,
			GroupID:   t.GroupID,
			IsActive:  t.IsActive,
			ExpiresAt: t.ExpiresAt,
			CreatedAt: t.CreatedAt,
		})
	}

	out := &ListTokensOutput{}
	out.Body = response

	return out, nil
}

func (h *TokenManagementHandler) DeleteToken(ctx context.Context, input *DeleteTokenInput) (*struct{}, error) {
	if err := h.tokenRepo.Deactivate(ctx, input.ID); err != nil {
		h.logger.Error("failed to deactivate token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to deactivate token")
	}

	return nil, nil
}
