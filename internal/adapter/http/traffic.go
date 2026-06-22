package http

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
)

// TrafficHandler exposes token-specific traffic and quota endpoints.
type TrafficHandler struct {
	trafficRepo domain.TrafficRepository
	tokenRepo   domain.TokenRepository
	logger      *slog.Logger
}

// NewTrafficHandler constructs a traffic handler.
func NewTrafficHandler(
	trafficRepo domain.TrafficRepository,
	tokenRepo domain.TokenRepository,
	logger *slog.Logger,
) *TrafficHandler {
	return &TrafficHandler{
		trafficRepo: trafficRepo,
		tokenRepo:   tokenRepo,
		logger:      logger,
	}
}

type TrafficListInput struct {
	ID     string `path:"id" required:"true"`
	Period string `query:"period" enum:"day,month" doc:"Aggregation period"`
	Limit  int    `query:"limit" default:"30" doc:"Number of periods to return"`
}

type TrafficItem struct {
	PeriodStart   time.Time `json:"period_start"`
	UploadBytes   int64     `json:"upload_bytes"`
	DownloadBytes int64     `json:"download_bytes"`
	TotalBytes    int64     `json:"total_bytes"`
}

type TrafficListOutput struct {
	Body []TrafficItem `json:"traffic"`
}

type QuotaUpdateInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		QuotaBytes  *int64 `json:"quota_bytes"`
		QuotaPeriod string `json:"quota_period" enum:"day,month," example:"month"`
	}
}

// Register wires traffic endpoints into Huma API.
func (h *TrafficHandler) Register(api huma.API) {
	huma.Get(api, "/v1/tokens/{id}/traffic", h.ListTokenTraffic)
	huma.Patch(api, "/v1/tokens/{id}/quota", h.UpdateQuota)
}

// ListTokenTraffic returns historical traffic for a token.
func (h *TrafficHandler) ListTokenTraffic(ctx context.Context, input *TrafficListInput) (*TrafficListOutput, error) {
	periodType := input.Period
	if periodType == "" {
		periodType = "day"
	}

	_, err := h.tokenRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return nil, huma.Error404NotFound("token not found")
		}
		h.logger.Error("failed to find token for traffic", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch token")
	}

	records, err := h.trafficRepo.ListUsageByToken(ctx, input.ID, periodType, input.Limit)
	if err != nil {
		h.logger.Error("failed to list token traffic", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch traffic")
	}

	out := &TrafficListOutput{}
	out.Body = make([]TrafficItem, 0, len(records))
	for _, r := range records {
		out.Body = append(out.Body, TrafficItem{
			PeriodStart:   r.PeriodStart,
			UploadBytes:   r.UploadBytes,
			DownloadBytes: r.DownloadBytes,
			TotalBytes:    r.UploadBytes + r.DownloadBytes,
		})
	}
	return out, nil
}

// UpdateQuota changes only the quota fields of a token.
func (h *TrafficHandler) UpdateQuota(ctx context.Context, input *QuotaUpdateInput) (*struct{}, error) {
	_, err := h.tokenRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return nil, huma.Error404NotFound("token not found")
		}
		h.logger.Error("failed to find token for quota update", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch token")
	}

	if err := h.tokenRepo.SetQuota(ctx, input.ID, input.Body.QuotaBytes, input.Body.QuotaPeriod); err != nil {
		h.logger.Error("failed to set token quota", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update quota")
	}
	return nil, nil
}
