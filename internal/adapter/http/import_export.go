package http

import (
	"context"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
)

// ImportExportHandler handles configuration export and import.
type ImportExportHandler struct {
	nodeRepo         domain.NodeRepository
	tokenRepo        domain.TokenRepository
	groupRepo        domain.GroupRepository
	publicSourceRepo domain.PublicSourceRepository
	inboundRepo      domain.InboundRepository
	logger           *slog.Logger
}

// NewImportExportHandler constructs an import/export handler.
func NewImportExportHandler(
	nodeRepo domain.NodeRepository,
	tokenRepo domain.TokenRepository,
	groupRepo domain.GroupRepository,
	publicSourceRepo domain.PublicSourceRepository,
	inboundRepo domain.InboundRepository,
	logger *slog.Logger,
) *ImportExportHandler {
	return &ImportExportHandler{
		nodeRepo:         nodeRepo,
		tokenRepo:        tokenRepo,
		groupRepo:        groupRepo,
		publicSourceRepo: publicSourceRepo,
		inboundRepo:      inboundRepo,
		logger:           logger,
	}
}

// exportNode is a serializable node representation.
type exportNode struct {
	ID       string   `json:"id"`
	URL      string   `json:"url"`
	GroupIDs []string `json:"group_ids"`
	Country  string   `json:"country"`
	IsSelf   bool     `json:"is_self"`
}

// exportGroup is a serializable group representation.
type exportGroup struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RandomEnabled bool   `json:"random_enabled"`
	RandomLimit   *int   `json:"random_limit,omitempty"`
}

// exportInbound is a serializable inbound representation.
type exportInbound struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	Port         int    `json:"port"`
	SNI          string `json:"sni"`
	Handshake    string `json:"handshake"`
	PublicKey    string `json:"public_key"`
	PrivateKey   string `json:"private_key"`
	ShortID      string `json:"short_id"`
	Fingerprint  string `json:"fingerprint"`
	NameTemplate string `json:"name_template"`
}

// exportPublicSource is a serializable public source representation.
type exportPublicSource struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	GroupID string `json:"group_id"`
}

// exportToken is a serializable token representation without secrets.
type exportToken struct {
	Owner       string   `json:"owner"`
	GroupIDs    []string `json:"group_ids"`
	InboundIDs  []string `json:"inbound_ids"`
	IsActive    bool     `json:"is_active"`
	QuotaBytes  *int64   `json:"quota_bytes,omitempty"`
	QuotaPeriod string   `json:"quota_period"`
	ExpiresAt   string   `json:"expires_at"`
}

// ExportOutput holds the full configuration export.
type ExportOutput struct {
	Body struct {
		Nodes         []exportNode         `json:"nodes"`
		Groups        []exportGroup        `json:"groups"`
		Inbounds      []exportInbound      `json:"inbounds"`
		PublicSources []exportPublicSource `json:"public_sources"`
		Tokens        []exportToken        `json:"tokens"`
	}
}

// ImportInput accepts a full configuration to import.
type ImportInput struct {
	Body struct {
		Nodes         []exportNode         `json:"nodes"`
		Groups        []exportGroup        `json:"groups"`
		Inbounds      []exportInbound      `json:"inbounds"`
		PublicSources []exportPublicSource `json:"public_sources"`
		Tokens        []exportToken        `json:"tokens"`
	}
}

// Register wires import/export endpoints into Huma API.
func (h *ImportExportHandler) Register(api huma.API) {
	huma.Get(api, "/v1/export", h.Export)
	huma.Post(api, "/v1/import", h.Import)
}

// Export dumps the current database configuration.
func (h *ImportExportHandler) Export(ctx context.Context, _ *struct{}) (*ExportOutput, error) {
	out := &ExportOutput{}

	nodeItems, err := h.exportNodes(ctx)
	if err != nil {
		return nil, err
	}
	out.Body.Nodes = nodeItems

	groupItems, err := h.exportGroups(ctx)
	if err != nil {
		return nil, err
	}
	out.Body.Groups = groupItems

	inboundItems, err := h.exportInbounds(ctx)
	if err != nil {
		return nil, err
	}
	out.Body.Inbounds = inboundItems

	psItems, err := h.exportPublicSources(ctx)
	if err != nil {
		return nil, err
	}
	out.Body.PublicSources = psItems

	tokenItems, err := h.exportTokens(ctx)
	if err != nil {
		return nil, err
	}
	out.Body.Tokens = tokenItems

	return out, nil
}

func (h *ImportExportHandler) exportNodes(ctx context.Context) ([]exportNode, error) {
	nodes, err := h.nodeRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list nodes for export", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to export nodes")
	}
	items := make([]exportNode, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, exportNode{
			ID:       n.ID,
			URL:      n.URL,
			GroupIDs: n.GroupIDs,
			Country:  n.Country,
			IsSelf:   n.IsSelf,
		})
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
		})
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

// Import loads a full configuration into the database.
func (h *ImportExportHandler) Import(ctx context.Context, input *ImportInput) (*struct{}, error) {
	h.importGroups(ctx, input.Body.Groups)
	h.importNodes(ctx, input.Body.Nodes)
	h.importInbounds(ctx, input.Body.Inbounds)
	h.importPublicSources(ctx, input.Body.PublicSources)
	h.importTokens(ctx, input.Body.Tokens)

	h.logger.Info("configuration imported",
		slog.Int("groups", len(input.Body.Groups)),
		slog.Int("nodes", len(input.Body.Nodes)),
		slog.Int("inbounds", len(input.Body.Inbounds)),
		slog.Int("public_sources", len(input.Body.PublicSources)),
		slog.Int("tokens", len(input.Body.Tokens)),
	)
	return nil, nil
}

func (h *ImportExportHandler) importGroups(ctx context.Context, groups []exportGroup) {
	for _, g := range groups {
		if err := h.groupRepo.Create(ctx, domain.Group{
			ID:            g.ID,
			Name:          g.Name,
			RandomEnabled: g.RandomEnabled,
			RandomLimit:   g.RandomLimit,
			CreatedAt:     time.Now().UTC(),
		}); err != nil {
			h.logger.Warn("import group skipped", slog.String("id", g.ID), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) importNodes(ctx context.Context, nodes []exportNode) {
	for _, n := range nodes {
		if err := h.nodeRepo.Upsert(ctx, domain.Node{
			ID:       n.ID,
			URL:      n.URL,
			GroupIDs: n.GroupIDs,
			Country:  n.Country,
			IsSelf:   n.IsSelf,
		}); err != nil {
			h.logger.Warn("import node skipped", slog.String("id", n.ID), slog.String("error", err.Error()))
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
