package http

import (
	"context"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

func (h *ImportExportHandler) exportNodes(ctx context.Context) ([]exportNode, error) {
	nodes, err := h.nodeRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list nodes for export", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to export nodes")
	}
	items := make([]exportNode, 0, len(nodes))
	for _, n := range nodes {
		item := exportNode{
			ID:       n.ID,
			URL:      n.URL,
			GroupIDs: n.GroupIDs,
			Country:  n.Country,
			IsSelf:   n.IsSelf,
		}
		if n.CountryInfo != nil {
			item.CountryCode = n.CountryInfo.CountryCode
			item.CountryName = n.CountryInfo.CountryName
			item.CountryFlag = n.CountryInfo.Flag
		}
		if n.ExpiresAt != nil {
			item.ExpiresAt = n.ExpiresAt.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items, nil
}

func (h *ImportExportHandler) exportGroups(ctx context.Context) ([]exportGroup, error) {
	groups, err := h.groupRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list groups for export", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to export groups")
	}
	items := make([]exportGroup, 0, len(groups))
	for _, g := range groups {
		items = append(items, exportGroup{
			ID:            g.ID,
			Name:          g.Name,
			RandomEnabled: g.RandomEnabled,
			RandomLimit:   g.RandomLimit,
			IsTopUp:       g.IsTopUp,
		})
	}
	return items, nil
}

func (h *ImportExportHandler) exportTopUps(ctx context.Context) ([]exportTopUp, error) {
	topUps, err := h.topUpRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list top-ups for export", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to export top-ups")
	}

	items := make([]exportTopUp, 0, len(topUps))
	for _, t := range topUps {
		item := exportTopUp{
			GroupID:      t.GroupID,
			URLs:         t.URLs,
			ParserType:   t.ParserType,
			ParserParams: t.ParserParams,
			CheckEnabled: t.CheckEnabled,
			CheckConfig:  fromTopUpCheckConfig(t.CheckConfig),
			ScheduleType: t.ScheduleType,
			ScheduleExpr: t.ScheduleExpr,
			NextRunAt:    t.NextRunAt.UTC().Format(time.RFC3339),
			Enabled:      t.Enabled,
		}
		if t.LastRunAt != nil {
			item.LastRunAt = t.LastRunAt.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items, nil
}

func (h *ImportExportHandler) exportInbounds(ctx context.Context) ([]exportInbound, error) {
	inbounds, err := h.inboundRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list inbounds for export", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to export inbounds")
	}
	items := make([]exportInbound, 0, len(inbounds))
	for _, i := range inbounds {
		items = append(items, exportInbound{
			ID:           i.ID,
			Name:         i.Name,
			Address:      i.Address,
			Port:         i.Port,
			SNI:          i.SNI,
			Handshake:    i.Handshake,
			PublicKey:    i.PublicKey,
			PrivateKey:   i.PrivateKey,
			ShortID:      i.ShortID,
			Fingerprint:  i.Fingerprint,
			NameTemplate: i.NameTemplate,
		})
	}
	return items, nil
}

func (h *ImportExportHandler) exportPublicSources(ctx context.Context) ([]exportPublicSource, error) {
	publicSources, err := h.publicSourceRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list public sources for export", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to export public sources")
	}
	items := make([]exportPublicSource, 0, len(publicSources))
	for _, ps := range publicSources {
		items = append(items, exportPublicSource{
			ID:      ps.ID,
			URL:     ps.URL,
			GroupID: ps.GroupID,
		})
	}
	return items, nil
}

func (h *ImportExportHandler) exportTokens(ctx context.Context) ([]exportToken, error) {
	tokens, err := h.tokenRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list tokens for export", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to export tokens")
	}
	items := make([]exportToken, 0, len(tokens))
	for _, t := range tokens {
		items = append(items, exportToken{
			Owner:       t.Owner,
			GroupIDs:    t.GroupIDs,
			InboundIDs:  t.InboundIDs,
			IsActive:    t.IsActive,
			QuotaBytes:  t.QuotaBytes,
			QuotaPeriod: t.QuotaPeriod,
			ExpiresAt:   t.ExpiresAt.Format(time.RFC3339),
		})
	}
	return items, nil
}
