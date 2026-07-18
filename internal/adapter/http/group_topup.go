package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
	"outless/internal/service"
)

// GroupTopUpManagementHandler exposes direct top-up management endpoints.
type GroupTopUpManagementHandler struct {
	topUpRepo domain.GroupTopUpRepository
	groupRepo domain.GroupRepository
	scheduler *service.TopUpScheduler
	logger    *slog.Logger
}

// NewGroupTopUpManagementHandler constructs a handler for direct top-up management.
func NewGroupTopUpManagementHandler(
	topUpRepo domain.GroupTopUpRepository,
	groupRepo domain.GroupRepository,
	scheduler *service.TopUpScheduler,
	logger *slog.Logger,
) *GroupTopUpManagementHandler {
	return &GroupTopUpManagementHandler{
		topUpRepo: topUpRepo,
		groupRepo: groupRepo,
		scheduler: scheduler,
		logger:    logger,
	}
}

func (h *GroupTopUpManagementHandler) Register(api huma.API) {
	huma.Get(api, "/v1/group-top-ups", h.ListTopUps)
	huma.Get(api, "/v1/group-top-ups/{id}", h.GetTopUp)
	huma.Post(api, "/v1/group-top-ups/ping", h.PingSourceURL)
	huma.Post(api, "/v1/group-top-ups/run", h.RunAllTopUps)
	huma.Post(api, "/v1/group-top-ups/{id}/run", h.RunTopUp)
	huma.Delete(api, "/v1/group-top-ups/{id}", h.DeleteTopUp)
}

func (h *GroupTopUpManagementHandler) ListTopUps(ctx context.Context, _ *struct{}) (*ListTopUpsOutput, error) {
	topUps, err := h.topUpRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list top-ups", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list top-ups")
	}

	items := make([]TopUpOutput, 0, len(topUps))
	for _, t := range topUps {
		items = append(items, toTopUpOutput(t))
	}
	return &ListTopUpsOutput{Body: items}, nil
}

func (h *GroupTopUpManagementHandler) GetTopUp(ctx context.Context, input *GetTopUpInput) (*GetTopUpOutput, error) {
	topUp, err := h.topUpRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrGroupTopUpNotFound) {
			return nil, huma.Error404NotFound("top-up not found")
		}
		h.logger.Error("failed to find top-up", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find top-up")
	}
	return &GetTopUpOutput{Body: toTopUpOutput(topUp)}, nil
}

func (h *GroupTopUpManagementHandler) RunTopUp(ctx context.Context, input *RunTopUpInput) (*struct{}, error) {
	if h.scheduler == nil {
		return nil, huma.Error500InternalServerError("scheduler not available")
	}
	topUp, err := h.topUpRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrGroupTopUpNotFound) {
			return nil, huma.Error404NotFound("top-up not found")
		}
		h.logger.Error("failed to find top-up for run", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find top-up")
	}

	go func() {
		if _, err := h.scheduler.RunNow(context.Background(), topUp.ID); err != nil {
			h.logger.Error("failed to run top-up", slog.String("id", input.ID), slog.String("error", err.Error()))
		}
	}()

	return nil, nil
}

func (h *GroupTopUpManagementHandler) RunAllTopUps(ctx context.Context, _ *RunAllTopUpsInput) (*RunAllTopUpsOutput, error) {
	if h.scheduler == nil {
		return nil, huma.Error500InternalServerError("scheduler not available")
	}

	runResults := h.scheduler.RunAll(ctx)
	output := make([]TopUpRunResultOutput, 0, len(runResults))
	for _, r := range runResults {
		status := "ok"
		if r.Failed {
			status = "failed"
		}
		groupName := ""
		if group, err := h.groupRepo.FindByID(ctx, r.GroupID); err == nil {
			groupName = group.Name
		}
		output = append(output, TopUpRunResultOutput{
			TopUpID:   r.TopUpID,
			GroupID:   r.GroupID,
			GroupName: groupName,
			Status:    status,
			Total:     r.Total,
			Passed:    r.Passed,
			Added:     r.Added,
			Error:     r.Error,
		})
	}

	return &RunAllTopUpsOutput{Body: output}, nil
}

func (h *GroupTopUpManagementHandler) DeleteTopUp(ctx context.Context, input *DeleteTopUpInput) (*struct{}, error) {
	topUp, err := h.topUpRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrGroupTopUpNotFound) {
			return nil, huma.Error404NotFound("top-up not found")
		}
		h.logger.Error("failed to find top-up for delete", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find top-up")
	}

	if err := h.topUpRepo.Delete(ctx, input.ID); err != nil {
		h.logger.Error("failed to delete top-up", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete top-up")
	}

	group, err := h.groupRepo.FindByID(ctx, topUp.GroupID)
	if err == nil && group.IsTopUp {
		group.IsTopUp = false
		if err := h.groupRepo.Update(ctx, group); err != nil {
			h.logger.Error("failed to clear group top-up flag", slog.String("group_id", group.ID), slog.String("error", err.Error()))
		}
	}

	return nil, nil
}

// PingSourceURLInput carries a URL to be checked for reachability.
type PingSourceURLInput struct {
	Body struct {
		URL string `json:"url" required:"true"`
	}
}

// PingSourceURLOutput reports the reachability check result.
type PingSourceURLOutput struct {
	Body struct {
		OK        bool   `json:"ok"`
		LatencyMS int64  `json:"latency_ms"`
		Error     string `json:"error,omitempty"`
	}
}

// PingSourceURL performs a lightweight HTTP GET against the supplied URL and reports whether it is reachable.
func (h *GroupTopUpManagementHandler) PingSourceURL(ctx context.Context, input *PingSourceURLInput) (*PingSourceURLOutput, error) {
	if input.Body.URL == "" {
		return nil, huma.Error400BadRequest("url is required")
	}

	client := &http.Client{Timeout: 15 * time.Second}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, input.Body.URL, nil)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	resp, err := client.Do(req)
	elapsed := time.Since(start).Milliseconds()

	out := &PingSourceURLOutput{}
	if err != nil {
		out.Body.OK = false
		out.Body.LatencyMS = elapsed
		out.Body.Error = err.Error()
		return out, nil
	}
	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out.Body.OK = false
		out.Body.LatencyMS = elapsed
		out.Body.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return out, nil
	}

	out.Body.OK = true
	out.Body.LatencyMS = elapsed
	return out, nil
}
