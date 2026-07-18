package http

import (
	"context"
	"log/slog"
	"time"

	"outless/internal/domain"
)

func (h *ImportExportHandler) importGroups(ctx context.Context, groups []exportGroup) {
	for _, g := range groups {
		if err := h.groupRepo.Create(ctx, domain.Group{
			ID:            g.ID,
			Name:          g.Name,
			RandomEnabled: g.RandomEnabled,
			RandomLimit:   g.RandomLimit,
			IsTopUp:       g.IsTopUp,
			CreatedAt:     time.Now().UTC(),
		}); err != nil {
			h.logger.Warn("import group skipped", slog.String("id", g.ID), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) importNodes(ctx context.Context, nodes []exportNode) {
	topUpGroupIDs := h.topUpGroupIDs(ctx)

	for _, n := range nodes {
		if hasTopUpGroup(n.GroupIDs, topUpGroupIDs) {
			h.logger.Warn("import node skipped: targets a top-up group", slog.String("id", n.ID))
			continue
		}

		node := domain.Node{
			ID:       n.ID,
			URL:      n.URL,
			GroupIDs: n.GroupIDs,
			Country:  n.Country,
			IsSelf:   n.IsSelf,
		}
		if n.ExpiresAt != "" {
			expiresAt, err := time.Parse(time.RFC3339, n.ExpiresAt)
			if err != nil {
				h.logger.Warn("import node skipped", slog.String("id", n.ID), slog.String("error", err.Error()))
				continue
			}
			node.ExpiresAt = &expiresAt
		}
		if err := h.nodeRepo.Upsert(ctx, node); err != nil {
			h.logger.Warn("import node skipped", slog.String("id", n.ID), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) importTopUps(ctx context.Context, topUps []exportTopUp) {
	for _, t := range topUps {
		id, err := domain.GenerateGroupTopUpID()
		if err != nil {
			h.logger.Warn("import top-up skipped", slog.String("group_id", t.GroupID), slog.String("error", err.Error()))
			continue
		}

		cfg, err := toTopUpCheckConfig(t.CheckConfig)
		if err != nil {
			h.logger.Warn("import top-up skipped", slog.String("group_id", t.GroupID), slog.String("error", err.Error()))
			continue
		}

		nextRun := time.Now().UTC()
		if t.NextRunAt != "" {
			if p, err := time.Parse(time.RFC3339, t.NextRunAt); err == nil {
				nextRun = p.UTC()
			}
		}

		var lastRun *time.Time
		if t.LastRunAt != "" {
			if p, err := time.Parse(time.RFC3339, t.LastRunAt); err == nil {
				lr := p.UTC()
				lastRun = &lr
			}
		}

		topUp := domain.GroupTopUp{
			ID:           id,
			GroupID:      t.GroupID,
			URLs:         t.URLs,
			ParserType:   t.ParserType,
			ParserParams: t.ParserParams,
			CheckEnabled: t.CheckEnabled,
			CheckConfig:  cfg,
			ScheduleType: t.ScheduleType,
			ScheduleExpr: t.ScheduleExpr,
			NextRunAt:    nextRun,
			LastRunAt:    lastRun,
			Enabled:      t.Enabled,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if err := h.topUpRepo.Create(ctx, topUp); err != nil {
			h.logger.Warn("import top-up skipped", slog.String("group_id", t.GroupID), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) importInbounds(ctx context.Context, inbounds []exportInbound) {
	for _, i := range inbounds {
		if err := h.inboundRepo.Create(ctx, domain.Inbound{
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
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}); err != nil {
			h.logger.Warn("import inbound skipped", slog.String("id", i.ID), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) importPublicSources(ctx context.Context, sources []exportPublicSource) {
	for _, ps := range sources {
		if err := h.publicSourceRepo.Create(ctx, domain.PublicSource{
			ID:        ps.ID,
			URL:       ps.URL,
			GroupID:   ps.GroupID,
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			h.logger.Warn("import public source skipped", slog.String("id", ps.ID), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) importTokens(ctx context.Context, tokens []exportToken) {
	for _, t := range tokens {
		expiresAt, _ := time.Parse(time.RFC3339, t.ExpiresAt)
		if expiresAt.IsZero() {
			expiresAt = time.Now().UTC().Add(30 * 24 * time.Hour)
		}
		if _, _, err := h.tokenRepo.IssueToken(
			ctx, t.Owner, t.GroupIDs, t.InboundIDs, expiresAt, t.QuotaBytes, t.QuotaPeriod,
		); err != nil {
			h.logger.Warn("import token skipped", slog.String("owner", t.Owner), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) topUpGroupIDs(ctx context.Context) map[string]struct{} {
	groups, err := h.groupRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list groups for import guard", slog.String("error", err.Error()))
		return nil
	}
	ids := make(map[string]struct{})
	for _, g := range groups {
		if g.IsTopUp {
			ids[g.ID] = struct{}{}
		}
	}
	return ids
}

func hasTopUpGroup(groupIDs []string, topUpGroupIDs map[string]struct{}) bool {
	for _, id := range groupIDs {
		if _, ok := topUpGroupIDs[id]; ok {
			return true
		}
	}
	return false
}
