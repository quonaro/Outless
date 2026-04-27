package http

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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
		Owner     string   `json:"owner" required:"true" maxLength:"64"`
		GroupIDs  []string `json:"group_ids"`
		ExpiresIn string   `json:"expires_in" example:"24h"`
	}
}

type CreateTokenOutput struct {
	Body struct {
		ID        string    `json:"id"`
		Token     string    `json:"token"`
		AccessURL string    `json:"access_url"`
		Owner     string    `json:"owner"`
		GroupID   string    `json:"group_id"`
		GroupIDs  []string  `json:"group_ids"`
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

type UpdateTokenInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		Owner     string   `json:"owner" required:"true" maxLength:"64"`
		GroupIDs  []string `json:"group_ids"`
		ExpiresIn string   `json:"expires_in" example:"24h"`
	}
}

type TokenItem struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	GroupID   string    `json:"group_id"`
	GroupIDs  []string  `json:"group_ids"`
	AccessURL string    `json:"access_url"`
	IsActive  bool      `json:"is_active"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *TokenManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/tokens", h.CreateToken)
	huma.Get(api, "/v1/tokens", h.ListTokens)
	huma.Put(api, "/v1/tokens/{id}", h.UpdateToken)
	huma.Post(api, "/v1/tokens/{id}/deactivate", h.DeactivateToken)
	huma.Post(api, "/v1/tokens/{id}/activate", h.ActivateToken)
	huma.Delete(api, "/v1/tokens/{id}", h.RemoveToken)
}

func (h *TokenManagementHandler) CreateToken(ctx context.Context, input *CreateTokenInput) (*CreateTokenOutput, error) {
	if input.Body.Owner == "" {
		return nil, huma.Error400BadRequest("owner is required")
	}

	groupIDs := uniqueStringSlice(input.Body.GroupIDs)

	// Validate explicitly selected groups.
	for _, groupID := range groupIDs {
		if _, err := h.groupRepo.FindByID(ctx, groupID); err != nil {
			if errors.Is(err, domain.ErrNodeNotFound) {
				h.logger.Warn("group not found", slog.String("group_id", groupID))
				return nil, huma.Error400BadRequest("group not found")
			}
			h.logger.Error("failed to find group", slog.String("group_id", groupID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to validate group")
		}
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
	token, err := h.tokenRepo.IssueToken(ctx, input.Body.Owner, groupIDs, expiresAt)
	if err != nil {
		h.logger.Error("failed to issue token", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create token")
	}

	out := &CreateTokenOutput{}
	out.Body.ID = token.ID
	out.Body.Token = token.TokenPlain
	out.Body.AccessURL = "/v1/sub/" + token.TokenPlain
	out.Body.Owner = token.Owner
	out.Body.GroupID = token.GroupID
	out.Body.GroupIDs = token.GroupIDs
	out.Body.IsActive = token.IsActive
	out.Body.ExpiresAt = token.ExpiresAt
	out.Body.CreatedAt = token.CreatedAt

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
			GroupIDs:  t.GroupIDs,
			AccessURL: tokenAccessURL(t.TokenPlain),
			IsActive:  t.IsActive,
			ExpiresAt: t.ExpiresAt,
			CreatedAt: t.CreatedAt,
		})
	}

	out := &ListTokensOutput{}
	out.Body = response

	return out, nil
}

func (h *TokenManagementHandler) DeactivateToken(ctx context.Context, input *DeleteTokenInput) (*struct{}, error) {
	if err := h.tokenRepo.Deactivate(ctx, input.ID); err != nil {
		h.logger.Error("failed to deactivate token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to deactivate token")
	}

	return nil, nil
}

func (h *TokenManagementHandler) ActivateToken(ctx context.Context, input *DeleteTokenInput) (*struct{}, error) {
	if err := h.tokenRepo.Activate(ctx, input.ID); err != nil {
		h.logger.Error("failed to activate token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to activate token")
	}

	return nil, nil
}

func (h *TokenManagementHandler) RemoveToken(ctx context.Context, input *DeleteTokenInput) (*struct{}, error) {
	if err := h.tokenRepo.Remove(ctx, input.ID); err != nil {
		h.logger.Error("failed to remove token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to remove token")
	}

	return nil, nil
}

func (h *TokenManagementHandler) UpdateToken(ctx context.Context, input *UpdateTokenInput) (*struct{}, error) {
	if input.Body.Owner == "" {
		return nil, huma.Error400BadRequest("owner is required")
	}

	groupIDs := uniqueStringSlice(input.Body.GroupIDs)

	// Validate explicitly selected groups.
	for _, groupID := range groupIDs {
		if _, err := h.groupRepo.FindByID(ctx, groupID); err != nil {
			if errors.Is(err, domain.ErrNodeNotFound) {
				h.logger.Warn("group not found", slog.String("group_id", groupID))
				return nil, huma.Error400BadRequest("group not found")
			}
			h.logger.Error("failed to find group", slog.String("group_id", groupID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to validate group")
		}
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
	if err := h.tokenRepo.Update(ctx, input.ID, input.Body.Owner, groupIDs, expiresAt); err != nil {
		h.logger.Error("failed to update token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update token")
	}

	return nil, nil
}

func uniqueStringSlice(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func tokenAccessURL(tokenPlain string) string {
	if tokenPlain == "" {
		return ""
	}
	return "/v1/sub/" + tokenPlain
}
