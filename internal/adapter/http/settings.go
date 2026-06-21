package http

import (
	"context"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"outless/shared/config"
)

// SettingsHandler manages safe, non-secret server settings via YAML config.
type SettingsHandler struct {
	configPath string
	logger     *slog.Logger
}

// NewSettingsHandler constructs a settings handler.
func NewSettingsHandler(configPath string, logger *slog.Logger) *SettingsHandler {
	return &SettingsHandler{
		configPath: configPath,
		logger:     logger,
	}
}

// SafeAPIConfig exposes app settings.
type SafeAPIConfig struct {
	ShutdownGracetime string `json:"shutdown_gracetime"`
	HTTPPort          int    `json:"http_port"`
	LogLevel          string `json:"log_level"`
	DisableDocs       bool   `json:"disable_docs"`
}

// SettingsOutput is returned by GET /v1/settings.
type SettingsOutput struct {
	Body struct {
		Database config.Database `json:"database"`
		App      SafeAPIConfig   `json:"app"`
	}
}

// UpdateSettingsInput is accepted by PUT /v1/settings.
type UpdateSettingsInput struct {
	Body struct {
		Database config.Database `json:"database"`
		App      SafeAPIConfig   `json:"app"`
	}
}

// Register wires settings endpoints into Huma API.
func (h *SettingsHandler) Register(api huma.API) {
	huma.Get(api, "/v1/settings", h.GetSettings)
	huma.Put(api, "/v1/settings", h.UpdateSettings)
}

// GetSettings returns non-sensitive settings loaded from the YAML file.
func (h *SettingsHandler) GetSettings(ctx context.Context, _ *struct{}) (*SettingsOutput, error) {
	loader := config.NewLoader(h.logger)
	cfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to load config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to load settings")
	}

	out := &SettingsOutput{}
	out.Body.Database = cfg.Database
	out.Body.App = SafeAPIConfig{
		ShutdownGracetime: cfg.App.ShutdownGracetime.String(),
		HTTPPort:          cfg.App.HTTPPort,
		LogLevel:          cfg.App.LogLevel,
		DisableDocs:       cfg.App.DisableDocs,
	}
	return out, nil
}

// UpdateSettings persists non-sensitive settings, preserving JWT secret.
func (h *SettingsHandler) UpdateSettings(ctx context.Context, input *UpdateSettingsInput) (*struct{}, error) {
	loader := config.NewLoader(h.logger)
	cfg := config.DefaultConfig()
	if err := loader.LoadOrCreate(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to load current config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to load current settings")
	}

	cfg.Database = input.Body.Database
	if d := config.ParseDuration(input.Body.App.ShutdownGracetime, cfg.App.ShutdownGracetime); d > 0 {
		cfg.App.ShutdownGracetime = d
	}
	cfg.App.HTTPPort = input.Body.App.HTTPPort
	cfg.App.DisableDocs = input.Body.App.DisableDocs
	cfg.App.LogLevel = input.Body.App.LogLevel
	if err := loader.Save(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to save config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to save settings")
	}

	h.logger.Info("settings updated and saved")
	return nil, nil
}
