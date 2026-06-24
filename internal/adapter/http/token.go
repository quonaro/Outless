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
	huma.Get(api, "/v1/tokens/{id}/ips", h.ListIPRestrictions)
	huma.Post(api, "/v1/tokens/{id}/ips", h.AddIPRestriction)
	huma.Delete(api, "/v1/tokens/{id}/ips/{ip}", h.RemoveIPRestriction)
	huma.Post(api, "/v1/tokens/batch-deactivate", h.BatchDeactivateTokens)
	huma.Post(api, "/v1/tokens/batch-delete", h.BatchRemoveTokens)
	huma.Post(api, "/v1/tokens/{id}/reissue", h.ReissueToken)
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
			AccessURL:       t.AccessURL,
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

type reissueTokenInput struct {
	ID string `path:"id" required:"true"`
}

type reissueTokenOutput struct {
	Body struct {
		ID        string `json:"id"`
		Token     string `json:"token"`
		AccessURL string `json:"access_url"`
		Owner     string `json:"owner"`
	}
}

func (h *TokenManagementHandler) ReissueToken(ctx context.Context, input *reissueTokenInput) (*reissueTokenOutput, error) {
	token, plainToken, err := h.tokenRepo.ReissueToken(ctx, input.ID)
	if err != nil {
		h.logger.Error("failed to reissue token", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to reissue token")
	}

	if err := h.runtime.ForceSync(); err != nil {
		h.logger.Warn("failed to sync after reissue", slog.String("error", err.Error()))
	}

	out := &reissueTokenOutput{}
	out.Body.ID = token.ID
	out.Body.Token = plainToken
	out.Body.AccessURL = token.AccessURL
	out.Body.Owner = token.Owner
	return out, nil
}

type ipRestrictionItem struct {
	IP   string `json:"ip"`
	Mode string `json:"mode"`
}

type listIPRestrictionsOutput struct {
	Body []ipRestrictionItem `json:"restrictions"`
}

func (h *TokenManagementHandler) ListIPRestrictions(ctx context.Context, input *DeleteTokenInput) (*listIPRestrictionsOutput, error) {
	restrictions, err := h.tokenRepo.ListIPRestrictions(ctx, input.ID)
	if err != nil {
		h.logger.Error("failed to list ip restrictions", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list ip restrictions")
	}
	out := make([]ipRestrictionItem, 0, len(restrictions))
	for _, r := range restrictions {
		out = append(out, ipRestrictionItem{IP: r.IP, Mode: r.Mode})
	}
	return &listIPRestrictionsOutput{Body: out}, nil
}

type addIPRestrictionInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		IP   string `json:"ip" required:"true" maxLength:"45"`
		Mode string `json:"mode" required:"true" enum:"allow,block"`
	}
}

func (h *TokenManagementHandler) AddIPRestriction(ctx context.Context, input *addIPRestrictionInput) (*struct{}, error) {
	if err := h.tokenRepo.AddIPRestriction(ctx, input.ID, input.Body.IP, input.Body.Mode); err != nil {
		h.logger.Error("failed to add ip restriction", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to add ip restriction")
	}
	h.logger.Info("ip restriction added", slog.String("id", input.ID), slog.String("ip", input.Body.IP), slog.String("mode", input.Body.Mode))
	return nil, nil
}

type removeIPRestrictionInput struct {
	ID string `path:"id" required:"true"`
	IP string `path:"ip" required:"true" maxLength:"45"`
}

func (h *TokenManagementHandler) RemoveIPRestriction(ctx context.Context, input *removeIPRestrictionInput) (*struct{}, error) {
	if err := h.tokenRepo.RemoveIPRestriction(ctx, input.ID, input.IP); err != nil {
		h.logger.Error("failed to remove ip restriction", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to remove ip restriction")
	}
	h.logger.Info("ip restriction removed", slog.String("id", input.ID), slog.String("ip", input.IP))
	return nil, nil
}

type batchTokenIDsInput struct {
	Body struct {
		IDs []string `json:"ids" required:"true"`
	}
}

func (h *TokenManagementHandler) BatchDeactivateTokens(ctx context.Context, input *batchTokenIDsInput) (*struct{}, error) {
	return h.processBatchTokens(ctx, input.Body.IDs, "deactivate", h.tokenRepo.Deactivate)
}

func (h *TokenManagementHandler) BatchRemoveTokens(ctx context.Context, input *batchTokenIDsInput) (*struct{}, error) {
	return h.processBatchTokens(ctx, input.Body.IDs, "remove", h.tokenRepo.Remove)
}

func (h *TokenManagementHandler) processBatchTokens(
	ctx context.Context,
	ids []string,
	action string,
	handler func(context.Context, string) error,
) (*struct{}, error) {
	if len(ids) == 0 {
		return nil, huma.Error400BadRequest("ids are required")
	}
	for _, id := range ids {
		if err := handler(ctx, id); err != nil {
			h.logger.Error(
				"failed to process token in batch",
				slog.String("action", action), slog.String("id", id), slog.String("error", err.Error()),
			)
		}
		if err := h.runtime.RemoveUser(id); err != nil {
			h.logger.Warn("failed to remove user from runtime", slog.String("id", id), slog.String("error", err.Error()))
		}
	}
	h.logger.Info("batch processed tokens", slog.String("action", action), slog.Int("count", len(ids)))
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
