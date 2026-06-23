package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
)

// RuntimeController interface for token lifecycle management.
type RuntimeController interface {
	RemoveUser(email string) error
	RemoveRulesForUser(email string) error
	ForceSync() error
}

type TokenManagementHandler struct {
	tokenRepo   domain.TokenRepository
	groupRepo   domain.GroupRepository
	nodeRepo    domain.NodeRepository
	inboundRepo domain.InboundRepository
	runtime     RuntimeController
	logger      *slog.Logger
}

func NewTokenManagementHandler(
	tokenRepo domain.TokenRepository,
	groupRepo domain.GroupRepository,
	nodeRepo domain.NodeRepository,
	inboundRepo domain.InboundRepository,
	runtime RuntimeController,
	logger *slog.Logger,
) *TokenManagementHandler {
	return &TokenManagementHandler{
		tokenRepo:   tokenRepo,
		groupRepo:   groupRepo,
		nodeRepo:    nodeRepo,
		inboundRepo: inboundRepo,
		runtime:     runtime,
		logger:      logger,
	}
}

type CreateTokenInput struct {
	Body struct {
		Owner       string   `json:"owner" required:"true" maxLength:"64"`
		GroupIDs    []string `json:"group_ids"`
		InboundIDs  []string `json:"inbound_ids"`
		ExpiresIn   string   `json:"expires_in" example:"24h"`
		QuotaBytes  *int64   `json:"quota_bytes,omitempty"`
		QuotaPeriod string   `json:"quota_period,omitempty" example:"month"`
	}
}

type CreateTokenOutput struct {
	Body struct {
		ID          string    `json:"id"`
		Token       string    `json:"token"`
		AccessURL   string    `json:"access_url"`
		Owner       string    `json:"owner"`
		GroupID     string    `json:"group_id"`
		GroupIDs    []string  `json:"group_ids"`
		InboundIDs  []string  `json:"inbound_ids"`
		IsActive    bool      `json:"is_active"`
		QuotaBytes  *int64    `json:"quota_bytes,omitempty"`
		QuotaPeriod string    `json:"quota_period"`
		ExpiresAt   time.Time `json:"expires_at"`
		CreatedAt   time.Time `json:"created_at"`
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
		Owner       string   `json:"owner" required:"true" maxLength:"64"`
		GroupIDs    []string `json:"group_ids"`
		InboundIDs  []string `json:"inbound_ids"`
		ExpiresIn   string   `json:"expires_in" example:"24h"`
		QuotaBytes  *int64   `json:"quota_bytes,omitempty"`
		QuotaPeriod string   `json:"quota_period,omitempty" example:"month"`
	}
}

type TokenItem struct {
	ID              string    `json:"id"`
	Owner           string    `json:"owner"`
	GroupID         string    `json:"group_id"`
	GroupIDs        []string  `json:"group_ids"`
	InboundIDs      []string  `json:"inbound_ids"`
	AccessURL       string    `json:"access_url"`
	IsActive        bool      `json:"is_active"`
	QuotaBytes      *int64    `json:"quota_bytes,omitempty"`
	QuotaPeriod     string    `json:"quota_period"`
	UsedBytes       int64     `json:"used_bytes"`
	LastConnectedAt time.Time `json:"last_connected_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	CreatedAt       time.Time `json:"created_at"`
}

func (h *TokenManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/tokens", h.CreateToken)
	huma.Get(api, "/v1/tokens", h.ListTokens)
	huma.Put(api, "/v1/tokens/{id}", h.UpdateToken)
	huma.Post(api, "/v1/tokens/{id}/deactivate", h.DeactivateToken)
	huma.Post(api, "/v1/tokens/{id}/activate", h.ActivateToken)
	huma.Post(api, "/v1/tokens/{id}/reset-traffic", h.ResetTraffic)
	huma.Delete(api, "/v1/tokens/{id}", h.RemoveToken)
}

func (h *TokenManagementHandler) CreateToken(ctx context.Context, input *CreateTokenInput) (*CreateTokenOutput, error) {
	if input.Body.Owner == "" {
		return nil, huma.Error400BadRequest("owner is required")
	}

	groupIDs := uniqueStringSlice(input.Body.GroupIDs)
	inboundIDs := uniqueStringSlice(input.Body.InboundIDs)

	for _, groupID := range groupIDs {
		if _, err := h.groupRepo.FindByID(ctx, groupID); err != nil {
			if errors.Is(err, domain.ErrGroupNotFound) {
				h.logger.Warn("group not found", slog.String("group_id", groupID))
				return nil, huma.Error400BadRequest("group not found")
			}
			h.logger.Error("failed to find group", slog.String("group_id", groupID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to validate group")
		}
	}

	for _, inboundID := range inboundIDs {
		if _, err := h.inboundRepo.FindByID(ctx, inboundID); err != nil {
			if errors.Is(err, domain.ErrInboundNotFound) {
				h.logger.Warn("inbound not found", slog.String("inbound_id", inboundID))
				return nil, huma.Error400BadRequest("inbound not found")
			}
			h.logger.Error("failed to find inbound", slog.String("inbound_id", inboundID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to validate inbound")
		}
	}

	expiresIn := 30 * 24 * time.Hour
	if input.Body.ExpiresIn != "" {
		d, err := time.ParseDuration(input.Body.ExpiresIn)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid expires_in format")
		}
		expiresIn = d
	}

	expiresAt := time.Now().UTC().Add(expiresIn)
	token, plainToken, err := h.tokenRepo.IssueToken(
		ctx, input.Body.Owner, groupIDs, inboundIDs, expiresAt,
		input.Body.QuotaBytes, input.Body.QuotaPeriod,
	)
	if err != nil {
		h.logger.Error("failed to issue token", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create token")
	}

	out := &CreateTokenOutput{}
	out.Body.ID = token.ID
	out.Body.Token = plainToken
	out.Body.AccessURL = "/v1/sub/" + plainToken
	out.Body.Owner = token.Owner
	out.Body.GroupID = token.GroupID
	out.Body.GroupIDs = token.GroupIDs
	out.Body.InboundIDs = token.InboundIDs
	out.Body.IsActive = token.IsActive
	out.Body.QuotaBytes = token.QuotaBytes
	out.Body.QuotaPeriod = token.QuotaPeriod
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
			ID:              t.ID,
			Owner:           t.Owner,
			GroupID:         t.GroupID,
			GroupIDs:        t.GroupIDs,
			InboundIDs:      t.InboundIDs,
			IsActive:        t.IsActive,
			QuotaBytes:      t.QuotaBytes,
			QuotaPeriod:     t.QuotaPeriod,
			UsedBytes:       t.UsedBytes,
			LastConnectedAt: t.LastConnectedAt,
			ExpiresAt:       t.ExpiresAt,
			CreatedAt:       t.CreatedAt,
		})
	}

	out := &ListTokensOutput{}
	out.Body = response

	return out, nil
}

func (h *TokenManagementHandler) DeactivateToken(ctx context.Context, input *DeleteTokenInput) (*struct{}, error) {
	token, err := h.tokenRepo.FindByID(ctx, input.ID)
	if err != nil {
		h.logger.Error("failed to find token for deactivation",
			slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error404NotFound("token not found")
	}

	nodes, err := h.nodeRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list nodes for deactivation", slog.String("error", err.Error()))
	}

	baseEmail := fmt.Sprintf("token-%s@outless", token.ID)
	if err := h.runtime.RemoveRulesForUser(baseEmail); err != nil {
		h.logger.Warn("failed to remove rules for base client", slog.String("email", baseEmail), slog.String("error", err.Error()))
	}
	if err := h.runtime.RemoveUser(baseEmail); err != nil {
		h.logger.Warn("failed to remove base user from inbound", slog.String("email", baseEmail), slog.String("error", err.Error()))
	}

	for _, node := range nodes {
		nodeEmail := fmt.Sprintf("token-%s-node-%s@outless", token.ID, node.ID)
		if err := h.runtime.RemoveRulesForUser(nodeEmail); err != nil {
			h.logger.Warn("failed to remove rules for node client", slog.String("email", nodeEmail), slog.String("error", err.Error()))
		}
		if err := h.runtime.RemoveUser(nodeEmail); err != nil {
			h.logger.Warn("failed to remove node user from inbound", slog.String("email", nodeEmail), slog.String("error", err.Error()))
		}
	}

	if err := h.tokenRepo.Deactivate(ctx, input.ID); err != nil {
		h.logger.Error("failed to deactivate token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to deactivate token")
	}

	h.logger.Info("token deactivated, rules removed from runtime", slog.String("id", input.ID))
	return nil, nil
}

func (h *TokenManagementHandler) ActivateToken(ctx context.Context, input *DeleteTokenInput) (*struct{}, error) {
	if err := h.tokenRepo.Activate(ctx, input.ID); err != nil {
		h.logger.Error("failed to activate token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to activate token")
	}

	if err := h.runtime.ForceSync(); err != nil {
		h.logger.Warn("failed to sync after token activation", slog.String("error", err.Error()))
	}

	h.logger.Info("token activated and synced to runtime", slog.String("id", input.ID))
	return nil, nil
}

func (h *TokenManagementHandler) RemoveToken(ctx context.Context, input *DeleteTokenInput) (*struct{}, error) {
	token, err := h.tokenRepo.FindByID(ctx, input.ID)
	if err != nil {
		h.logger.Error("failed to find token for removal", slog.String("id", input.ID), slog.String("error", err.Error()))
	} else {
		nodes, err := h.nodeRepo.List(ctx)
		if err != nil {
			h.logger.Error("failed to list nodes for removal", slog.String("error", err.Error()))
		}

		baseEmail := fmt.Sprintf("token-%s@outless", token.ID)
		if err := h.runtime.RemoveRulesForUser(baseEmail); err != nil {
			h.logger.Warn("failed to remove rules for base client",
				slog.String("email", baseEmail), slog.String("error", err.Error()))
		}
		if err := h.runtime.RemoveUser(baseEmail); err != nil {
			h.logger.Warn("failed to remove base user from inbound",
				slog.String("email", baseEmail), slog.String("error", err.Error()))
		}

		for _, node := range nodes {
			nodeEmail := fmt.Sprintf("token-%s-node-%s@outless", token.ID, node.ID)
			if err := h.runtime.RemoveRulesForUser(nodeEmail); err != nil {
				h.logger.Warn("failed to remove rules for node client",
					slog.String("email", nodeEmail), slog.String("error", err.Error()))
			}
			if err := h.runtime.RemoveUser(nodeEmail); err != nil {
				h.logger.Warn("failed to remove node user from inbound",
					slog.String("email", nodeEmail), slog.String("error", err.Error()))
			}
		}
	}

	if err := h.tokenRepo.Remove(ctx, input.ID); err != nil {
		h.logger.Error("failed to remove token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to remove token")
	}

	h.logger.Info("token removed, users and rules removed from runtime", slog.String("id", input.ID))
	return nil, nil
}

func (h *TokenManagementHandler) UpdateToken(ctx context.Context, input *UpdateTokenInput) (*struct{}, error) {
	if input.Body.Owner == "" {
		return nil, huma.Error400BadRequest("owner is required")
	}

	groupIDs := uniqueStringSlice(input.Body.GroupIDs)
	inboundIDs := uniqueStringSlice(input.Body.InboundIDs)

	for _, groupID := range groupIDs {
		if _, err := h.groupRepo.FindByID(ctx, groupID); err != nil {
			if errors.Is(err, domain.ErrGroupNotFound) {
				h.logger.Warn("group not found", slog.String("group_id", groupID))
				return nil, huma.Error400BadRequest("group not found")
			}
			h.logger.Error("failed to find group", slog.String("group_id", groupID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to validate group")
		}
	}

	for _, inboundID := range inboundIDs {
		if _, err := h.inboundRepo.FindByID(ctx, inboundID); err != nil {
			if errors.Is(err, domain.ErrInboundNotFound) {
				h.logger.Warn("inbound not found", slog.String("inbound_id", inboundID))
				return nil, huma.Error400BadRequest("inbound not found")
			}
			h.logger.Error("failed to find inbound", slog.String("inbound_id", inboundID), slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to validate inbound")
		}
	}

	expiresIn := 30 * 24 * time.Hour
	if input.Body.ExpiresIn != "" {
		d, err := time.ParseDuration(input.Body.ExpiresIn)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid expires_in format")
		}
		expiresIn = d
	}

	expiresAt := time.Now().UTC().Add(expiresIn)
	if err := h.tokenRepo.Update(
		ctx, input.ID, input.Body.Owner, groupIDs, inboundIDs, expiresAt,
		input.Body.QuotaBytes, input.Body.QuotaPeriod,
	); err != nil {
		h.logger.Error("failed to update token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update token")
	}

	if h.runtime != nil {
		if err := h.runtime.ForceSync(); err != nil {
			h.logger.Warn("failed to force sync after token update",
				slog.String("id", input.ID), slog.String("error", err.Error()))
		}
	}

	return nil, nil
}

func (h *TokenManagementHandler) ResetTraffic(ctx context.Context, input *DeleteTokenInput) (*struct{}, error) {
	if err := h.tokenRepo.ResetTraffic(ctx, input.ID); err != nil {
		h.logger.Error("failed to reset token traffic", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to reset token traffic")
	}
	h.logger.Info("token traffic reset", slog.String("id", input.ID))
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
