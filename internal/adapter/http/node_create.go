package http

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
	"outless/shared/vless"
)

type CreateNodeInput struct {
	Body struct {
		URL       string   `json:"url"`
		GroupIDs  []string `json:"group_ids" required:"true"`
		IsSelf    bool     `json:"is_self"`
		ExpiresAt *string  `json:"expires_at,omitempty"`
	}
}

type CreateNodeOutput struct {
	Body struct {
		ID          string   `json:"id"`
		URL         string   `json:"url"`
		GroupIDs    []string `json:"group_ids"`
		Country     string   `json:"country,omitempty"`
		CountryCode string   `json:"country_code,omitempty"`
		CountryName string   `json:"country_name,omitempty"`
		CountryFlag string   `json:"country_flag,omitempty"`
		IsSelf      bool     `json:"is_self"`
		ExpiresAt   *string  `json:"expires_at,omitempty"`
	}
}

func (h *NodeManagementHandler) CreateNode(ctx context.Context, input *CreateNodeInput) (*CreateNodeOutput, error) {
	node, err := h.validateAndBuildCreateNode(ctx, input)
	if err != nil {
		return nil, err
	}

	if err := h.nodeRepo.Create(ctx, node); err != nil {
		if errors.Is(err, domain.ErrDuplicateNode) {
			return nil, huma.Error409Conflict("node already exists")
		}
		h.logger.Error("failed to create node", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create node")
	}

	countryInfo := h.resolveAndStoreCountryForNode(ctx, node)

	if err := h.runtime.ForceSync(); err != nil {
		h.logger.Warn("failed to sync after node creation", slog.String("error", err.Error()))
	}

	return newCreateNodeOutput(node, countryInfo), nil
}

func (h *NodeManagementHandler) validateAndBuildCreateNode(ctx context.Context, input *CreateNodeInput) (domain.Node, error) {
	if !input.Body.IsSelf && input.Body.URL == "" {
		return domain.Node{}, huma.Error400BadRequest("url is required when is_self is false")
	}

	if len(input.Body.GroupIDs) == 0 {
		return domain.Node{}, huma.Error400BadRequest("group_ids is required")
	}

	for _, groupID := range input.Body.GroupIDs {
		group, err := h.groupRepo.FindByID(ctx, groupID)
		if err != nil {
			if errors.Is(err, domain.ErrGroupNotFound) {
				h.logger.Warn("group not found", slog.String("group_id", groupID))
				return domain.Node{}, huma.Error400BadRequest("group not found")
			}
			h.logger.Error("failed to find group", slog.String("group_id", groupID), slog.String("error", err.Error()))
			return domain.Node{}, huma.Error500InternalServerError("failed to validate group")
		}
		if group.IsTopUp {
			return domain.Node{}, huma.Error400BadRequest("cannot add nodes to a top-up group")
		}
	}

	if input.Body.IsSelf {
		exists, err := h.nodeRepo.HasSelfNode(ctx)
		if err != nil {
			h.logger.Error("failed to check self node", slog.String("error", err.Error()))
			return domain.Node{}, huma.Error500InternalServerError("failed to validate self node")
		}
		if exists {
			return domain.Node{}, huma.Error409Conflict("self node already exists")
		}
	}

	nodeID := generateNodeID(input.Body.URL, input.Body.GroupIDs)
	if input.Body.IsSelf {
		nodeID = "self_" + strings.Join(input.Body.GroupIDs, "_")
	}

	expiresAt, err := parseOptionalExpiresAt(input.Body.ExpiresAt)
	if err != nil {
		return domain.Node{}, huma.Error400BadRequest("invalid expires_at")
	}

	return domain.Node{
		ID:        nodeID,
		URL:       input.Body.URL,
		GroupIDs:  input.Body.GroupIDs,
		IsSelf:    input.Body.IsSelf,
		ExpiresAt: expiresAt,
	}, nil
}

func (h *NodeManagementHandler) resolveAndStoreCountryForNode(ctx context.Context, node domain.Node) *domain.CountryInfo {
	if h.countryResolver == nil {
		return nil
	}
	if !node.IsSelf && node.URL == "" {
		return nil
	}

	countryInfo := h.resolveNodeCountry(ctx, node)
	if countryInfo == nil {
		return nil
	}

	if err := h.nodeRepo.UpdateCountryInfo(ctx, node.ID, countryInfo); err != nil {
		h.logger.Warn("failed to update country info after creation",
			slog.String("node_id", node.ID),
			slog.String("error", err.Error()),
		)
		return nil
	}

	return countryInfo
}

func (h *NodeManagementHandler) resolveNodeCountry(ctx context.Context, node domain.Node) *domain.CountryInfo {
	host := vless.ExtractIPFromVLESS(node.URL)
	if host == "" && node.IsSelf {
		host = h.externalHost
	}
	if host == "" {
		h.logger.Warn("failed to extract host from vless url", slog.String("node_id", node.ID))
		return nil
	}

	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ip := host
	if host != "" && net.ParseIP(host) == nil {
		resolved, err := h.countryResolver.ResolveHost(resolveCtx, host)
		if err != nil {
			h.logger.Warn("failed to resolve node host",
				slog.String("node_id", node.ID),
				slog.String("host", host),
				slog.String("error", err.Error()),
			)
			return nil
		}
		ip = resolved
	}

	info, err := h.countryResolver.Lookup(resolveCtx, ip)
	if err != nil {
		h.logger.Warn("country lookup failed",
			slog.String("node_id", node.ID),
			slog.String("ip", ip),
			slog.String("error", err.Error()),
		)
		return nil
	}

	now := time.Now().UTC()
	info.LastLookupAt = &now
	info.NextRetryAt = nil
	info.Attempts = 0
	info.LastError = ""
	return &info
}

func newCreateNodeOutput(node domain.Node, countryInfo *domain.CountryInfo) *CreateNodeOutput {
	out := &CreateNodeOutput{}
	out.Body.ID = node.ID
	out.Body.URL = node.URL
	out.Body.GroupIDs = node.GroupIDs
	out.Body.IsSelf = node.IsSelf
	out.Body.ExpiresAt = formatOptionalExpiresAt(node.ExpiresAt)
	if countryInfo != nil {
		out.Body.Country = domain.NormalizeCountryCode(countryInfo.CountryCode)
		out.Body.CountryCode = countryInfo.CountryCode
		out.Body.CountryName = countryInfo.CountryName
		out.Body.CountryFlag = countryInfo.Flag
	}
	return out
}
