package http

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"outless/internal/service"
	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
)

// SubscriptionHandler serves base64 VLESS subscriptions.
type SubscriptionHandler struct {
	service *service.SubscriptionService
	logger  *slog.Logger
}

// NewSubscriptionHandler constructs subscription HTTP handler.
func NewSubscriptionHandler(service *service.SubscriptionService, logger *slog.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{service: service, logger: logger}
}

type getSubscriptionInput struct {
	Token string `path:"token" maxLength:"128"`
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

	payload, err := h.service.BuildBase64VLESS(ctx, token)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid or expired token")
		}

		h.logger.Error("failed to build subscription", slog.String("token", token), slog.String("error", err.Error()))
		return nil, err
	}

	if payload == "" {
		return nil, huma.Error404NotFound("subscription is empty")
	}

	return &getSubscriptionOutput{
		ContentType: "text/plain",
		Body:        []byte(payload),
	}, nil
}
