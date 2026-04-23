package httpadapter

import (
	"context"
	"log/slog"

	"outless/pkg/config"

	"github.com/danielgtaylor/huma/v2"
)

type SettingsHandler struct {
	configPath string
	logger     *slog.Logger
}

func NewSettingsHandler(configPath string, logger *slog.Logger) *SettingsHandler {
	return &SettingsHandler{
		configPath: configPath,
		logger:     logger,
	}
}

type SettingsOutput struct {
	Body struct {
		Database config.DatabaseConfig `json:"database"`
		API      config.APIConfig      `json:"api"`
		Checker  config.CheckerConfig  `json:"checker"`
	}
}

type UpdateSettingsInput struct {
	Body struct {
		Database config.DatabaseConfig `json:"database"`
		API      config.APIConfig      `json:"api"`
		Checker  config.CheckerConfig  `json:"checker"`
	}
}

func (h *SettingsHandler) Register(api huma.API) {
	huma.Get(api, "/v1/settings", h.GetSettings)
	huma.Put(api, "/v1/settings", h.UpdateSettings)
}

func (h *SettingsHandler) GetSettings(ctx context.Context, _ *struct{}) (*SettingsOutput, error) {
	loader := config.NewLoader(h.logger)
	cfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to load config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to load settings")
	}

	out := &SettingsOutput{}
	out.Body.Database = cfg.Database
	out.Body.API = cfg.API
	out.Body.Checker = cfg.Checker

	return out, nil
}

func (h *SettingsHandler) UpdateSettings(ctx context.Context, input *UpdateSettingsInput) (*struct{}, error) {
	// Load current config to preserve values not being updated
	loader := config.NewLoader(h.logger)
	cfg := config.DefaultConfig()
	if err := loader.LoadOrCreate(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to load current config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to load current settings")
	}

	// Update only non-secret fields
	cfg.Database = input.Body.Database
	cfg.Checker.Workers = input.Body.Checker.Workers
	cfg.Checker.LatencyFilter = input.Body.Checker.LatencyFilter
	cfg.Checker.PublicRefreshInterval = input.Body.Checker.PublicRefreshInterval
	cfg.Checker.CheckInterval = input.Body.Checker.CheckInterval
	cfg.Checker.Xray = input.Body.Checker.Xray
	cfg.API.ShutdownTimeout = input.Body.API.ShutdownTimeout
	cfg.API.JWT.Expiry = input.Body.API.JWT.Expiry

	// Preserve JWT secret and admin credentials
	cfg.API.JWT.Secret = input.Body.API.JWT.Secret
	if cfg.API.JWT.Secret == "" {
		cfg.API.JWT.Secret = "CHANGE_ME_IN_PRODUCTION"
	}
	cfg.API.Admin = input.Body.API.Admin

	if err := loader.Save(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to save config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to save settings")
	}

	h.logger.Info("settings updated and saved")
	return nil, nil
}
