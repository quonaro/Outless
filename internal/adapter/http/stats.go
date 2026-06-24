package http

import (
	"context"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
)

// StatsHandler aggregates counts across entities for the dashboard.
type StatsHandler struct {
	nodeRepo    domain.NodeRepository
	tokenRepo   domain.TokenRepository
	groupRepo   domain.GroupRepository
	inboundRepo domain.InboundRepository
	trafficRepo domain.TrafficRepository
	logger      *slog.Logger
}

// NewStatsHandler constructs a stats handler.
func NewStatsHandler(
	nodeRepo domain.NodeRepository,
	tokenRepo domain.TokenRepository,
	groupRepo domain.GroupRepository,
	inboundRepo domain.InboundRepository,
	trafficRepo domain.TrafficRepository,
	logger *slog.Logger,
) *StatsHandler {
	return &StatsHandler{
		nodeRepo:    nodeRepo,
		tokenRepo:   tokenRepo,
		groupRepo:   groupRepo,
		inboundRepo: inboundRepo,
		trafficRepo: trafficRepo,
		logger:      logger,
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
	huma.Get(api, "/v1/stats/traffic", h.GetTrafficStats)
	huma.Get(api, "/v1/stats/traffic/tokens", h.GetTokenTrafficStats)
	huma.Get(api, "/v1/stats/traffic/nodes", h.GetNodeTrafficStats)
	huma.Get(api, "/v1/stats/traffic/inbounds", h.GetInboundTrafficStats)
	huma.Get(api, "/v1/stats/traffic/domains", h.GetDomainTrafficStats)
	huma.Get(api, "/v1/stats/traffic/domains/history", h.GetDomainTrafficHistory)
	huma.Delete(api, "/v1/stats/traffic/domains/history", h.ClearDomainHistory)
}

type TrafficStatsOutput struct {
	Body struct {
		DayUploadBytes     int64 `json:"day_upload_bytes"`
		DayDownloadBytes   int64 `json:"day_download_bytes"`
		MonthUploadBytes   int64 `json:"month_upload_bytes"`
		MonthDownloadBytes int64 `json:"month_download_bytes"`
	}
}

// GetTrafficStats returns aggregate traffic totals for the current day and month.
func (h *StatsHandler) GetTrafficStats(ctx context.Context, _ *struct{}) (*TrafficStatsOutput, error) {
	now := time.Now().UTC()

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dayUpload, dayDownload, err := h.trafficRepo.GetAggregateForPeriod(ctx, "day", dayStart)
	if err != nil {
		h.logger.Error("failed to get daily traffic aggregate", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch traffic stats")
	}

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthUpload, monthDownload, err := h.trafficRepo.GetAggregateForPeriod(ctx, "month", monthStart)
	if err != nil {
		h.logger.Error("failed to get monthly traffic aggregate", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch traffic stats")
	}

	out := &TrafficStatsOutput{}
	out.Body.DayUploadBytes = dayUpload
	out.Body.DayDownloadBytes = dayDownload
	out.Body.MonthUploadBytes = monthUpload
	out.Body.MonthDownloadBytes = monthDownload
	return out, nil
}

type TrafficEntityItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	UploadBytes   int64  `json:"upload_bytes"`
	DownloadBytes int64  `json:"download_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type EntityTrafficOutput struct {
	Body struct {
		Items []TrafficEntityItem `json:"items"`
	}
}

// GetTokenTrafficStats returns per-token traffic for the current day.
//
//nolint:dupl
func (h *StatsHandler) GetTokenTrafficStats(ctx context.Context, _ *struct{}) (*EntityTrafficOutput, error) {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	usageList, err := h.trafficRepo.ListTokenUsageForPeriod(ctx, "day", dayStart, 1000)
	if err != nil {
		h.logger.Error("failed to list token usage", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch token traffic")
	}

	tokens, err := h.tokenRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list tokens", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch tokens")
	}

	tokenName := make(map[string]string, len(tokens))
	for _, t := range tokens {
		tokenName[t.ID] = t.Owner
	}

	out := &EntityTrafficOutput{}
	out.Body.Items = make([]TrafficEntityItem, 0, len(usageList))
	for _, u := range usageList {
		out.Body.Items = append(out.Body.Items, TrafficEntityItem{
			ID:            u.TokenID,
			Name:          tokenName[u.TokenID],
			UploadBytes:   u.UploadBytes,
			DownloadBytes: u.DownloadBytes,
			TotalBytes:    u.UploadBytes + u.DownloadBytes,
		})
	}
	return out, nil
}

// GetNodeTrafficStats returns per-node traffic for the current day.
//
//nolint:dupl
func (h *StatsHandler) GetNodeTrafficStats(ctx context.Context, _ *struct{}) (*EntityTrafficOutput, error) {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	usageList, err := h.trafficRepo.ListNodeUsage(ctx, "day", dayStart, 1000)
	if err != nil {
		h.logger.Error("failed to list node usage", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch node traffic")
	}

	nodes, err := h.nodeRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list nodes", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch nodes")
	}

	nodeName := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeName[n.ID] = n.URL
	}

	out := &EntityTrafficOutput{}
	out.Body.Items = make([]TrafficEntityItem, 0, len(usageList))
	for _, u := range usageList {
		out.Body.Items = append(out.Body.Items, TrafficEntityItem{
			ID:            u.NodeID,
			Name:          nodeName[u.NodeID],
			UploadBytes:   u.UploadBytes,
			DownloadBytes: u.DownloadBytes,
			TotalBytes:    u.UploadBytes + u.DownloadBytes,
		})
	}
	return out, nil
}

// GetInboundTrafficStats returns per-inbound traffic for the current day.
//
//nolint:dupl
func (h *StatsHandler) GetInboundTrafficStats(ctx context.Context, _ *struct{}) (*EntityTrafficOutput, error) {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	usageList, err := h.trafficRepo.ListInboundUsage(ctx, "day", dayStart, 1000)
	if err != nil {
		h.logger.Error("failed to list inbound usage", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch inbound traffic")
	}

	inbounds, err := h.inboundRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list inbounds", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch inbounds")
	}

	inboundName := make(map[string]string, len(inbounds))
	for _, ib := range inbounds {
		inboundName[ib.ID] = ib.Name
	}

	out := &EntityTrafficOutput{}
	out.Body.Items = make([]TrafficEntityItem, 0, len(usageList))
	for _, u := range usageList {
		name := inboundName[u.InboundTag]
		if name == "" {
			name = u.InboundTag
		}
		out.Body.Items = append(out.Body.Items, TrafficEntityItem{
			ID:            u.InboundTag,
			Name:          name,
			UploadBytes:   u.UploadBytes,
			DownloadBytes: u.DownloadBytes,
			TotalBytes:    u.UploadBytes + u.DownloadBytes,
		})
	}
	return out, nil
}

// GetDomainTrafficStats returns per-domain traffic for the current day.
//
//nolint:dupl
func (h *StatsHandler) GetDomainTrafficStats(ctx context.Context, _ *struct{}) (*EntityTrafficOutput, error) {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	usageList, err := h.trafficRepo.ListDomainUsage(ctx, "day", dayStart, 1000)
	if err != nil {
		h.logger.Error("failed to list domain usage", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch domain traffic")
	}

	tokens, err := h.tokenRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list tokens", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch tokens")
	}

	tokenName := make(map[string]string, len(tokens))
	for _, t := range tokens {
		tokenName[t.ID] = t.Owner
	}

	out := &EntityTrafficOutput{}
	out.Body.Items = make([]TrafficEntityItem, 0, len(usageList))
	for _, u := range usageList {
		name := tokenName[u.TokenID]
		if name == "" {
			name = u.TokenID
		}
		out.Body.Items = append(out.Body.Items, TrafficEntityItem{
			ID:            u.Domain,
			Name:          name,
			UploadBytes:   u.UploadBytes,
			DownloadBytes: u.DownloadBytes,
			TotalBytes:    u.UploadBytes + u.DownloadBytes,
		})
	}
	return out, nil
}

// DomainHistoryInput defines query parameters for domain history.
type DomainHistoryInput struct {
	Days int `query:"days" default:"30" minimum:"1" maximum:"365"`
}

type HistoryDomainItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	UploadBytes   int64  `json:"upload_bytes"`
	DownloadBytes int64  `json:"download_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type HistoryNodeItem struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	UploadBytes   int64               `json:"upload_bytes"`
	DownloadBytes int64               `json:"download_bytes"`
	TotalBytes    int64               `json:"total_bytes"`
	Domains       []HistoryDomainItem `json:"domains"`
}

type HistoryUserItem struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	UploadBytes   int64              `json:"upload_bytes"`
	DownloadBytes int64              `json:"download_bytes"`
	TotalBytes    int64              `json:"total_bytes"`
	Nodes         []*HistoryNodeItem `json:"nodes"`
}

type DomainHierarchyOutput struct {
	Body struct {
		Items []*HistoryUserItem `json:"items"`
	}
}

func buildDomainHierarchy(
	usageList []domain.DomainUsage,
	tokenName map[string]string,
	nodeName map[string]string,
) []*HistoryUserItem {
	type nodeKey struct{ userID, nodeID string }
	userMap := make(map[string]*HistoryUserItem)
	nodeMap := make(map[nodeKey]*HistoryNodeItem)

	for _, u := range usageList {
		userID := u.TokenID
		userName := tokenName[userID]
		if userName == "" {
			userName = userID
		}

		nodeID := u.NodeID
		nodeNameStr := nodeName[nodeID]
		if nodeNameStr == "" {
			nodeNameStr = nodeID
		}

		if userMap[userID] == nil {
			userMap[userID] = &HistoryUserItem{
				ID:   userID,
				Name: userName,
			}
		}
		user := userMap[userID]

		nk := nodeKey{userID: userID, nodeID: nodeID}
		if nodeMap[nk] == nil {
			nodeMap[nk] = &HistoryNodeItem{
				ID:   nodeID,
				Name: nodeNameStr,
			}
			user.Nodes = append(user.Nodes, nodeMap[nk])
		}
		node := nodeMap[nk]

		node.Domains = append(node.Domains, HistoryDomainItem{
			ID:            u.Domain,
			Name:          u.Domain,
			UploadBytes:   u.UploadBytes,
			DownloadBytes: u.DownloadBytes,
			TotalBytes:    u.UploadBytes + u.DownloadBytes,
		})

		node.UploadBytes += u.UploadBytes
		node.DownloadBytes += u.DownloadBytes
		node.TotalBytes += u.UploadBytes + u.DownloadBytes
		user.UploadBytes += u.UploadBytes
		user.DownloadBytes += u.DownloadBytes
		user.TotalBytes += u.UploadBytes + u.DownloadBytes
	}

	out := make([]*HistoryUserItem, 0, len(userMap))
	for _, u := range userMap {
		out = append(out, u)
	}
	return out
}

// GetDomainTrafficHistory returns per-domain traffic aggregated over the last N days grouped by user -> node -> domain.
func (h *StatsHandler) GetDomainTrafficHistory(ctx context.Context, input *DomainHistoryInput) (*DomainHierarchyOutput, error) {
	usageList, err := h.trafficRepo.ListDomainUsageAggregateByUser(ctx, input.Days)
	if err != nil {
		h.logger.Error("failed to list domain usage history", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch domain history")
	}

	tokens, err := h.tokenRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list tokens", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch tokens")
	}

	nodes, err := h.nodeRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list nodes", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to fetch nodes")
	}

	tokenName := make(map[string]string, len(tokens))
	for _, t := range tokens {
		tokenName[t.ID] = t.Owner
	}

	nodeName := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeName[n.ID] = n.Country
	}

	out := &DomainHierarchyOutput{}
	out.Body.Items = buildDomainHierarchy(usageList, tokenName, nodeName)
	return out, nil
}

// ClearDomainHistory deletes all domain usage records.
func (h *StatsHandler) ClearDomainHistory(ctx context.Context, _ *struct{}) (*struct{}, error) {
	if err := h.trafficRepo.DeleteAllDomainUsage(ctx); err != nil {
		h.logger.Error("failed to clear domain history", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to clear domain history")
	}
	return nil, nil
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
