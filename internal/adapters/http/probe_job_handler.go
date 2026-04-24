package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
)

// ProbeJobHandler exposes probe jobs status for async probe UX.
type ProbeJobHandler struct {
	repo   domain.ProbeJobRepository
	logger *slog.Logger
}

// NewProbeJobHandler builds a probe job handler.
func NewProbeJobHandler(repo domain.ProbeJobRepository, logger *slog.Logger) *ProbeJobHandler {
	return &ProbeJobHandler{
		repo:   repo,
		logger: logger,
	}
}

type ProbeJobGetInput struct {
	ID string `path:"id" required:"true"`
}

type ProbeJobListInput struct {
	Status  string `query:"status"`
	GroupID string `query:"group_id"`
	Limit   int    `query:"limit"`
}

type ProbeJobOutput struct {
	ID          string     `json:"id"`
	BatchID     string     `json:"batch_id,omitempty"`
	NodeID      string     `json:"node_id"`
	GroupID     string     `json:"group_id,omitempty"`
	RequestedBy string     `json:"requested_by,omitempty"`
	Mode        string     `json:"mode"`
	ProbeURL    string     `json:"probe_url,omitempty"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

type ProbeJobGetOutput struct {
	Body ProbeJobOutput `json:"job"`
}

type ProbeJobListOutput struct {
	Body []ProbeJobOutput `json:"jobs"`
}

func (h *ProbeJobHandler) Register(api huma.API) {
	huma.Get(api, "/v1/probe-jobs/{id}", h.Get)
	huma.Get(api, "/v1/probe-jobs", h.List)
}

func (h *ProbeJobHandler) Get(ctx context.Context, input *ProbeJobGetInput) (*ProbeJobGetOutput, error) {
	job, err := h.repo.GetByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrProbeJobNotFound) {
			return nil, huma.Error404NotFound("probe job not found")
		}
		h.logger.Error("failed to get probe job", slog.String("job_id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to get probe job")
	}

	return &ProbeJobGetOutput{
		Body: toProbeJobOutput(job),
	}, nil
}

func (h *ProbeJobHandler) List(ctx context.Context, input *ProbeJobListInput) (*ProbeJobListOutput, error) {
	filter := domain.ProbeJobListFilter{
		Status:  parseProbeJobStatus(input.Status),
		GroupID: strings.TrimSpace(input.GroupID),
		Limit:   input.Limit,
	}
	jobs, err := h.repo.List(ctx, filter)
	if err != nil {
		h.logger.Error("failed to list probe jobs", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list probe jobs")
	}

	out := make([]ProbeJobOutput, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, toProbeJobOutput(job))
	}
	return &ProbeJobListOutput{Body: out}, nil
}

func parseProbeJobStatus(raw string) domain.ProbeJobStatus {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(domain.ProbeJobStatusPending):
		return domain.ProbeJobStatusPending
	case string(domain.ProbeJobStatusRunning):
		return domain.ProbeJobStatusRunning
	case string(domain.ProbeJobStatusSucceeded):
		return domain.ProbeJobStatusSucceeded
	case string(domain.ProbeJobStatusFailed):
		return domain.ProbeJobStatusFailed
	default:
		return ""
	}
}

func toProbeJobOutput(job domain.ProbeJob) ProbeJobOutput {
	return ProbeJobOutput{
		ID:          job.ID,
		BatchID:     job.BatchID,
		NodeID:      job.NodeID,
		GroupID:     job.GroupID,
		RequestedBy: job.RequestedBy,
		Mode:        string(job.Mode),
		ProbeURL:    job.ProbeURL,
		Status:      string(job.Status),
		Attempts:    job.Attempts,
		Error:       job.LastError,
		CreatedAt:   job.CreatedAt,
		StartedAt:   job.StartedAt,
		FinishedAt:  job.FinishedAt,
	}
}
