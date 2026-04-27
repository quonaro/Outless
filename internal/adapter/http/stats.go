package http

import (
	"context"
	"log/slog"

	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
)

// StatsHandler aggregates counts across entities for the dashboard.
type StatsHandler struct {
	nodeRepo  domain.NodeRepository
	tokenRepo domain.TokenRepository
	groupRepo domain.GroupRepository
	logger    *slog.Logger
}

// NewStatsHandler constructs a stats handler.
func NewStatsHandler(nodeRepo domain.NodeRepository, tokenRepo domain.TokenRepository, groupRepo domain.GroupRepository, logger *slog.Logger) *StatsHandler {
	return &StatsHandler{
		nodeRepo:  nodeRepo,
		tokenRepo: tokenRepo,
		groupRepo: groupRepo,
		logger:    logger,
	}
}

// StatsOutput is the JSON payload returned by GET /v1/stats.
type StatsOutput struct {
	Body struct {
		NodesTotal   int `json:"nodes_total"`
		TokensTotal  int `json:"tokens_total"`
		TokensActive int `json:"tokens_active"`
		GroupsTotal  int `json:"groups_total"`
	}
}

// Register wires stats endpoints into Huma API.
func (h *StatsHandler) Register(api huma.API) {
	huma.Get(api, "/v1/stats", h.GetStats)
}

// GetStats returns counters aggregated from node/token/group repositories.
func (h *StatsHandler) GetStats(ctx context.Context, _ *struct{}) (*StatsOutput, error) {
	nodes, err := h.nodeRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list nodes for stats", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to collect stats")
	}

	tokens, err := h.tokenRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list tokens for stats", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to collect stats")
	}

	groups, err := h.groupRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list groups for stats", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to collect stats")
	}

	out := &StatsOutput{}
	out.Body.NodesTotal = len(nodes)

	out.Body.TokensTotal = len(tokens)
	for _, token := range tokens {
		if token.IsActive {
			out.Body.TokensActive++
		}
	}

	out.Body.GroupsTotal = len(groups)

	return out, nil
}
