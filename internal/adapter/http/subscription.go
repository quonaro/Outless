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

// SubscriptionHandler serves base64 VLESS subscriptions.
type SubscriptionHandler struct {
	service   *service.SubscriptionService
	tokenRepo domain.TokenRepository
	logger    *slog.Logger
}

// NewSubscriptionHandler constructs subscription HTTP handler.
func NewSubscriptionHandler(
	service *service.SubscriptionService,
	tokenRepo domain.TokenRepository,
	logger *slog.Logger,
) *SubscriptionHandler {
	return &SubscriptionHandler{service: service, tokenRepo: tokenRepo, logger: logger}
}

type getSubscriptionInput struct {
	Token     string `path:"token" maxLength:"128"`
	InboundID string `query:"inbound_id" maxLength:"128"`
}

type getSubscriptionOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

// Register wires subscription endpoints into Huma API.
func (h *SubscriptionHandler) Register(api huma.API) {
	huma.Get(api, "/v1/sub/{token}", h.getSubscription)
	huma.Get(api, "/v1/sub/{token}/base64", h.getBase64Subscription)
	huma.Get(api, "/v1/sub/{token}/clash", h.getClashSubscription)
	huma.Get(api, "/v1/sub/{token}/json", h.getJSONSubscription)
	huma.Get(api, "/v1/sub/{token}/singbox", h.getSingBoxSubscription)
	huma.Get(api, "/v1/sub/{token}/v2ray", h.getV2RaySubscription)
	huma.Get(api, "/v1/sub/{token}/surge", h.getSurgeSubscription)
}

func (h *SubscriptionHandler) getSubscription(ctx context.Context, input *getSubscriptionInput) (*getSubscriptionOutput, error) {
	token := strings.TrimSpace(input.Token)
	if token == "" || strings.Contains(token, "/") {
		return nil, huma.Error400BadRequest("invalid token")
	}

	tok, err := h.tokenRepo.GetTokenByPlain(ctx, token, time.Now().UTC())
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired token")
	}

	// Check IP restrictions.
	clientIP := GetClientIP(ctx)
	if clientIP != "" {
		allowed, err := h.tokenRepo.CheckIPAllowed(ctx, tok.ID, clientIP)
		if err != nil {
			h.logger.Error("failed to check ip", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("ip check failed")
		}
		if !allowed {
			h.logger.Warn("subscription denied by ip restriction", slog.String("token_id", tok.ID), slog.String("ip", clientIP))
			return nil, huma.Error403Forbidden("access denied from this ip")
		}
	}

	payload, err := h.service.BuildBase64VLESS(ctx, token, input.InboundID)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid or expired token")
		}

		h.logger.Error("failed to build subscription", slog.String("token", token), slog.String("error", err.Error()))
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	if payload == "" {
		return nil, huma.Error404NotFound("subscription is empty")
	}

	return &getSubscriptionOutput{
		ContentType: "text/plain",
		Body:        []byte(payload),
	}, nil
}

func (h *SubscriptionHandler) getClashSubscription(ctx context.Context, input *getSubscriptionInput) (*getSubscriptionOutput, error) {
	token := strings.TrimSpace(input.Token)
	if token == "" || strings.Contains(token, "/") {
		return nil, huma.Error400BadRequest("invalid token")
	}

	tok, err := h.tokenRepo.GetTokenByPlain(ctx, token, time.Now().UTC())
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired token")
	}

	clientIP := GetClientIP(ctx)
	if clientIP != "" {
		allowed, err := h.tokenRepo.CheckIPAllowed(ctx, tok.ID, clientIP)
		if err != nil {
			h.logger.Error("failed to check ip", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("ip check failed")
		}
		if !allowed {
			h.logger.Warn("subscription denied by ip restriction", slog.String("token_id", tok.ID), slog.String("ip", clientIP))
			return nil, huma.Error403Forbidden("access denied from this ip")
		}
	}

	payload, err := h.service.BuildClashMetaYAML(ctx, token, input.InboundID)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid or expired token")
		}
		h.logger.Error("failed to build clash subscription", slog.String("token", token), slog.String("error", err.Error()))
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	if payload == "" {
		return nil, huma.Error404NotFound("subscription is empty")
	}

	return &getSubscriptionOutput{
		ContentType: "text/yaml",
		Body:        []byte(payload),
	}, nil
}

func (h *SubscriptionHandler) getSingBoxSubscription(ctx context.Context, input *getSubscriptionInput) (*getSubscriptionOutput, error) {
	token := strings.TrimSpace(input.Token)
	if token == "" || strings.Contains(token, "/") {
		return nil, huma.Error400BadRequest("invalid token")
	}

	tok, err := h.tokenRepo.GetTokenByPlain(ctx, token, time.Now().UTC())
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired token")
	}

	clientIP := GetClientIP(ctx)
	if clientIP != "" {
		allowed, err := h.tokenRepo.CheckIPAllowed(ctx, tok.ID, clientIP)
		if err != nil {
			h.logger.Error("failed to check ip", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("ip check failed")
		}
		if !allowed {
			h.logger.Warn("subscription denied by ip restriction", slog.String("token_id", tok.ID), slog.String("ip", clientIP))
			return nil, huma.Error403Forbidden("access denied from this ip")
		}
	}

	payload, err := h.service.BuildSingBoxJSON(ctx, token, input.InboundID)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid or expired token")
		}
		h.logger.Error("failed to build singbox subscription", slog.String("token", token), slog.String("error", err.Error()))
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	if payload == "" {
		return nil, huma.Error404NotFound("subscription is empty")
	}

	return &getSubscriptionOutput{
		ContentType: "application/json",
		Body:        []byte(payload),
	}, nil
}

func (h *SubscriptionHandler) getV2RaySubscription(ctx context.Context, input *getSubscriptionInput) (*getSubscriptionOutput, error) {
	token := strings.TrimSpace(input.Token)
	if token == "" || strings.Contains(token, "/") {
		return nil, huma.Error400BadRequest("invalid token")
	}

	tok, err := h.tokenRepo.GetTokenByPlain(ctx, token, time.Now().UTC())
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired token")
	}

	clientIP := GetClientIP(ctx)
	if clientIP != "" {
		allowed, err := h.tokenRepo.CheckIPAllowed(ctx, tok.ID, clientIP)
		if err != nil {
			h.logger.Error("failed to check ip", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("ip check failed")
		}
		if !allowed {
			h.logger.Warn("subscription denied by ip restriction", slog.String("token_id", tok.ID), slog.String("ip", clientIP))
			return nil, huma.Error403Forbidden("access denied from this ip")
		}
	}

	payload, err := h.service.BuildV2RayBase64(ctx, token, input.InboundID)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid or expired token")
		}
		h.logger.Error("failed to build v2ray subscription", slog.String("token", token), slog.String("error", err.Error()))
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	if payload == "" {
		return nil, huma.Error404NotFound("subscription is empty")
	}

	return &getSubscriptionOutput{
		ContentType: "text/plain",
		Body:        []byte(payload),
	}, nil
}

func (h *SubscriptionHandler) getSurgeSubscription(ctx context.Context, input *getSubscriptionInput) (*getSubscriptionOutput, error) {
	token := strings.TrimSpace(input.Token)
	if token == "" || strings.Contains(token, "/") {
		return nil, huma.Error400BadRequest("invalid token")
	}

	tok, err := h.tokenRepo.GetTokenByPlain(ctx, token, time.Now().UTC())
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired token")
	}

	clientIP := GetClientIP(ctx)
	if clientIP != "" {
		allowed, err := h.tokenRepo.CheckIPAllowed(ctx, tok.ID, clientIP)
		if err != nil {
			h.logger.Error("failed to check ip", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("ip check failed")
		}
		if !allowed {
			h.logger.Warn("subscription denied by ip restriction", slog.String("token_id", tok.ID), slog.String("ip", clientIP))
			return nil, huma.Error403Forbidden("access denied from this ip")
		}
	}

	payload, err := h.service.BuildSurgeConf(ctx, token, input.InboundID)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid or expired token")
		}
		h.logger.Error("failed to build surge subscription", slog.String("token", token), slog.String("error", err.Error()))
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	if payload == "" {
		return nil, huma.Error404NotFound("subscription is empty")
	}

	return &getSubscriptionOutput{
		ContentType: "text/plain; charset=utf-8",
		Body:        []byte(payload),
	}, nil
}

func (h *SubscriptionHandler) getBase64Subscription(ctx context.Context, input *getSubscriptionInput) (*getSubscriptionOutput, error) {
	token := strings.TrimSpace(input.Token)
	if token == "" || strings.Contains(token, "/") {
		return nil, huma.Error400BadRequest("invalid token")
	}

	tok, err := h.tokenRepo.GetTokenByPlain(ctx, token, time.Now().UTC())
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired token")
	}

	clientIP := GetClientIP(ctx)
	if clientIP != "" {
		allowed, err := h.tokenRepo.CheckIPAllowed(ctx, tok.ID, clientIP)
		if err != nil {
			h.logger.Error("failed to check ip", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("ip check failed")
		}
		if !allowed {
			h.logger.Warn("subscription denied by ip restriction", slog.String("token_id", tok.ID), slog.String("ip", clientIP))
			return nil, huma.Error403Forbidden("access denied from this ip")
		}
	}

	payload, err := h.service.BuildBase64VLESS(ctx, token, input.InboundID)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid or expired token")
		}
		h.logger.Error("failed to build base64 subscription", slog.String("token", token), slog.String("error", err.Error()))
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	if payload == "" {
		return nil, huma.Error404NotFound("subscription is empty")
	}

	return &getSubscriptionOutput{
		ContentType: "text/plain",
		Body:        []byte(payload),
	}, nil
}

func (h *SubscriptionHandler) getJSONSubscription(ctx context.Context, input *getSubscriptionInput) (*getSubscriptionOutput, error) {
	token := strings.TrimSpace(input.Token)
	if token == "" || strings.Contains(token, "/") {
		return nil, huma.Error400BadRequest("invalid token")
	}

	tok, err := h.tokenRepo.GetTokenByPlain(ctx, token, time.Now().UTC())
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired token")
	}

	clientIP := GetClientIP(ctx)
	if clientIP != "" {
		allowed, err := h.tokenRepo.CheckIPAllowed(ctx, tok.ID, clientIP)
		if err != nil {
			h.logger.Error("failed to check ip", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("ip check failed")
		}
		if !allowed {
			h.logger.Warn("subscription denied by ip restriction", slog.String("token_id", tok.ID), slog.String("ip", clientIP))
			return nil, huma.Error403Forbidden("access denied from this ip")
		}
	}

	payload, err := h.service.BuildSingBoxJSON(ctx, token, input.InboundID)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid or expired token")
		}
		h.logger.Error("failed to build json subscription", slog.String("token", token), slog.String("error", err.Error()))
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	if payload == "" {
		return nil, huma.Error404NotFound("subscription is empty")
	}

	return &getSubscriptionOutput{
		ContentType: "application/json",
		Body:        []byte(payload),
	}, nil
}
