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
