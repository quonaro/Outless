package http

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
	"outless/shared/config"
)

type InboundManagementHandler struct {
	inboundRepo domain.InboundRepository
	logger      *slog.Logger
}

func NewInboundManagementHandler(inboundRepo domain.InboundRepository, logger *slog.Logger) *InboundManagementHandler {
	return &InboundManagementHandler{inboundRepo: inboundRepo, logger: logger}
}

type InboundItem struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Address            string    `json:"address"`
	Port               int       `json:"port"`
	SNI                string    `json:"sni"`
	Handshake          string    `json:"handshake"`
	PublicKey          string    `json:"public_key"`
	ShortID            string    `json:"short_id"`
	Fingerprint        string    `json:"fingerprint"`
	URLHost            string    `json:"url_host"`
	NameTemplate       string    `json:"name_template"`
	EnableAutoSelfNode bool      `json:"enable_auto_self_node"`
	AutoSelfNodeName   string    `json:"auto_self_node_name"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateInboundInput struct {
	Body struct {
		Name               string `json:"name" required:"true" maxLength:"100"`
		Address            string `json:"address" required:"false"`
		Port               int    `json:"port" required:"false"`
		SNI                string `json:"sni" required:"false"`
		Handshake          string `json:"handshake" required:"false"`
		PrivateKey         string `json:"private_key" required:"false"`
		ShortID            string `json:"short_id" required:"false"`
		Fingerprint        string `json:"fingerprint" required:"false"`
		URLHost            string `json:"url_host" required:"false"`
		NameTemplate       string `json:"name_template" required:"false"`
		EnableAutoSelfNode bool   `json:"enable_auto_self_node" required:"false"`
		AutoSelfNodeName   string `json:"auto_self_node_name" required:"false"`
	}
}

type CreateInboundOutput struct {
	Body InboundItem
}

type ListInboundsOutput struct {
	Body []InboundItem `json:"inbounds"`
}

type UpdateInboundInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		Name               string `json:"name" required:"true" maxLength:"100"`
		Address            string `json:"address" required:"false"`
		Port               int    `json:"port" required:"false"`
		SNI                string `json:"sni" required:"false"`
		Handshake          string `json:"handshake" required:"false"`
		PrivateKey         string `json:"private_key" required:"false"`
		ShortID            string `json:"short_id" required:"false"`
		Fingerprint        string `json:"fingerprint" required:"false"`
		URLHost            string `json:"url_host" required:"false"`
		NameTemplate       string `json:"name_template" required:"false"`
		EnableAutoSelfNode bool   `json:"enable_auto_self_node" required:"false"`
		AutoSelfNodeName   string `json:"auto_self_node_name" required:"false"`
	}
}

type DeleteInboundInput struct {
	ID string `path:"id" required:"true"`
}

func (h *InboundManagementHandler) Register(api huma.API) {
	huma.Post(api, "/v1/inbounds", h.CreateInbound)
	huma.Get(api, "/v1/inbounds", h.ListInbounds)
	huma.Put(api, "/v1/inbounds/{id}", h.UpdateInbound)
	huma.Delete(api, "/v1/inbounds/{id}", h.DeleteInbound)
}

func (h *InboundManagementHandler) CreateInbound(ctx context.Context, input *CreateInboundInput) (*CreateInboundOutput, error) {
	input.Body.Name = strings.TrimSpace(input.Body.Name)
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("name is required")
	}

	id, err := domain.GenerateInboundID()
	if err != nil {
		h.logger.Error("failed to generate inbound id", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create inbound")
	}

	priv := input.Body.PrivateKey
	pub := ""
	if priv != "" {
		pub, err = config.DeriveRealityPublicKey(priv)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid private key")
		}
	} else {
		priv, pub, err = config.GenerateRealityKeyPair()
		if err != nil {
			h.logger.Error("failed to generate reality key pair", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to generate reality key pair")
		}
	}

	shortID := strings.TrimSpace(input.Body.ShortID)
	if shortID == "" {
		shortID, err = config.GenerateRealityShortID()
		if err != nil {
			h.logger.Error("failed to generate reality short_id", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to generate reality short_id")
		}
	}

	now := time.Now().UTC()
	inbound := domain.Inbound{
		ID:                 id,
		Name:               input.Body.Name,
		Address:            strings.TrimSpace(input.Body.Address),
		Port:               input.Body.Port,
		SNI:                strings.TrimSpace(input.Body.SNI),
		Handshake:          strings.TrimSpace(input.Body.Handshake),
		PrivateKey:         priv,
		PublicKey:          pub,
		ShortID:            shortID,
		Fingerprint:        strings.TrimSpace(input.Body.Fingerprint),
		URLHost:            strings.TrimSpace(input.Body.URLHost),
		NameTemplate:       input.Body.NameTemplate,
		EnableAutoSelfNode: input.Body.EnableAutoSelfNode,
		AutoSelfNodeName:   input.Body.AutoSelfNodeName,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if inbound.Port == 0 {
		inbound.Port = 443
	}
	if inbound.Fingerprint == "" {
		inbound.Fingerprint = "chrome"
	}
	if inbound.AutoSelfNodeName == "" {
		inbound.AutoSelfNodeName = "Direct Exit"
	}
	if inbound.Handshake == "" {
		inbound.Handshake = inbound.SNI
	}

	if err := h.inboundRepo.Create(ctx, inbound); err != nil {
		h.logger.Error("failed to create inbound", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create inbound")
	}

	out := &CreateInboundOutput{}
	out.Body = toInboundItem(inbound)
	return out, nil
}

func (h *InboundManagementHandler) ListInbounds(ctx context.Context, _ *struct{}) (*ListInboundsOutput, error) {
	inbounds, err := h.inboundRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list inbounds", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list inbounds")
	}

	items := make([]InboundItem, 0, len(inbounds))
	for _, inbound := range inbounds {
		items = append(items, toInboundItem(inbound))
	}

	out := &ListInboundsOutput{}
	out.Body = items
	return out, nil
}

func (h *InboundManagementHandler) UpdateInbound(ctx context.Context, input *UpdateInboundInput) (*struct{}, error) {
	input.Body.Name = strings.TrimSpace(input.Body.Name)
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("name is required")
	}

	inbound, err := h.inboundRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrInboundNotFound) {
			return nil, huma.Error404NotFound("inbound not found")
		}
		h.logger.Error("failed to find inbound", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find inbound")
	}

	inbound.Name = input.Body.Name
	inbound.Address = strings.TrimSpace(input.Body.Address)
	inbound.Port = input.Body.Port
	inbound.SNI = strings.TrimSpace(input.Body.SNI)
	inbound.Handshake = strings.TrimSpace(input.Body.Handshake)
	if strings.TrimSpace(input.Body.ShortID) == "" && inbound.ShortID == "" {
		shortID, err := config.GenerateRealityShortID()
		if err != nil {
			h.logger.Error("failed to generate reality short_id", slog.String("error", err.Error()))
			return nil, huma.Error500InternalServerError("failed to generate reality short_id")
		}
		inbound.ShortID = shortID
	} else if strings.TrimSpace(input.Body.ShortID) != "" {
		inbound.ShortID = strings.TrimSpace(input.Body.ShortID)
	}
	inbound.Fingerprint = strings.TrimSpace(input.Body.Fingerprint)
	inbound.URLHost = strings.TrimSpace(input.Body.URLHost)
	inbound.NameTemplate = input.Body.NameTemplate
	inbound.EnableAutoSelfNode = input.Body.EnableAutoSelfNode
	inbound.AutoSelfNodeName = input.Body.AutoSelfNodeName

	if inbound.Port == 0 {
		inbound.Port = 443
	}
	if inbound.Fingerprint == "" {
		inbound.Fingerprint = "chrome"
	}
	if inbound.AutoSelfNodeName == "" {
		inbound.AutoSelfNodeName = "Direct Exit"
	}
	if inbound.Handshake == "" {
		inbound.Handshake = inbound.SNI
	}

	if input.Body.PrivateKey != "" && input.Body.PrivateKey != inbound.PrivateKey {
		pub, err := config.DeriveRealityPublicKey(input.Body.PrivateKey)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid private key")
		}
		inbound.PrivateKey = input.Body.PrivateKey
		inbound.PublicKey = pub
	}

	if err := h.inboundRepo.Update(ctx, inbound); err != nil {
		h.logger.Error("failed to update inbound", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update inbound")
	}
	return nil, nil
}

func (h *InboundManagementHandler) DeleteInbound(ctx context.Context, input *DeleteInboundInput) (*struct{}, error) {
	if err := h.inboundRepo.Delete(ctx, input.ID); err != nil {
		if errors.Is(err, domain.ErrInboundNotFound) {
			return nil, huma.Error404NotFound("inbound not found")
		}
		h.logger.Error("failed to delete inbound", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete inbound")
	}
	return nil, nil
}

func toInboundItem(inbound domain.Inbound) InboundItem {
	return InboundItem{
		ID:                 inbound.ID,
		Name:               inbound.Name,
		Address:            inbound.Address,
		Port:               inbound.Port,
		SNI:                inbound.SNI,
		Handshake:          inbound.Handshake,
		PublicKey:          inbound.PublicKey,
		ShortID:            inbound.ShortID,
		Fingerprint:        inbound.Fingerprint,
		URLHost:            inbound.URLHost,
		NameTemplate:       inbound.NameTemplate,
		EnableAutoSelfNode: inbound.EnableAutoSelfNode,
		AutoSelfNodeName:   inbound.AutoSelfNodeName,
		CreatedAt:          inbound.CreatedAt,
		UpdatedAt:          inbound.UpdatedAt,
	}
}
