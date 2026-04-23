package httpadapter

import (
	"context"
	"log/slog"

	"outless/pkg/config"

	"github.com/danielgtaylor/huma/v2"
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

// SafeAPIConfig exposes only non-sensitive API settings.
// JWT secret and admin credentials are intentionally omitted.
type SafeAPIConfig struct {
	ShutdownTimeout string          `json:"shutdown_timeout"`
	JWTExpiry       string          `json:"jwt_expiry"`
	AdminLogin      string          `json:"admin_login"`
	Hub             config.HubConfig `json:"hub"`
}

// SafeCheckerConfig exposes checker settings without secrets.
type SafeCheckerConfig struct {
	Workers               int              `json:"workers"`
	LatencyFilter         string           `json:"latency_filter"`
	PublicRefreshInterval string           `json:"public_refresh_interval"`
	CheckInterval         string           `json:"check_interval"`
	Xray                  config.XrayConfig `json:"xray"`
}

// SettingsOutput is returned by GET /v1/settings.
type SettingsOutput struct {
	Body struct {
		Database config.DatabaseConfig `json:"database"`
		API      SafeAPIConfig         `json:"api"`
		Checker  SafeCheckerConfig     `json:"checker"`
	}
}

// UpdateSettingsInput is accepted by PUT /v1/settings.
type UpdateSettingsInput struct {
	Body struct {
		Database config.DatabaseConfig `json:"database"`
		API      struct {
			ShutdownTimeout string          `json:"shutdown_timeout"`
			JWTExpiry       string          `json:"jwt_expiry"`
			Hub             config.HubConfig `json:"hub"`
		} `json:"api"`
		Checker SafeCheckerConfig `json:"checker"`
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
	out.Body.API = SafeAPIConfig{
		ShutdownTimeout: cfg.API.ShutdownTimeout.String(),
		JWTExpiry:       cfg.API.JWT.Expiry.String(),
		AdminLogin:      cfg.API.Admin.Login,
		Hub:             cfg.Hub,
	}
	out.Body.Checker = SafeCheckerConfig{
		Workers:               cfg.Checker.Workers,
		LatencyFilter:         cfg.Checker.LatencyFilter.String(),
		PublicRefreshInterval: cfg.Checker.PublicRefreshInterval.String(),
		CheckInterval:         cfg.Checker.CheckInterval.String(),
		Xray:                  cfg.Checker.Xray,
	}

	return out, nil
}

// UpdateSettings persists non-sensitive settings, preserving JWT secret and admin bootstrap credentials.
func (h *SettingsHandler) UpdateSettings(ctx context.Context, input *UpdateSettingsInput) (*struct{}, error) {
	loader := config.NewLoader(h.logger)
	cfg := config.DefaultConfig()
	if err := loader.LoadOrCreate(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to load current config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to load current settings")
	}

	cfg.Database = input.Body.Database

	if d := config.ParseDuration(input.Body.API.ShutdownTimeout, cfg.API.ShutdownTimeout); d > 0 {
		cfg.API.ShutdownTimeout = d
	}
	if d := config.ParseDuration(input.Body.API.JWTExpiry, cfg.API.JWT.Expiry); d > 0 {
		cfg.API.JWT.Expiry = d
	}
	cfg.Hub = input.Body.API.Hub

	cfg.Checker.Workers = input.Body.Checker.Workers
	if d := config.ParseDuration(input.Body.Checker.LatencyFilter, cfg.Checker.LatencyFilter); d > 0 {
		cfg.Checker.LatencyFilter = d
	}
	if d := config.ParseDuration(input.Body.Checker.PublicRefreshInterval, cfg.Checker.PublicRefreshInterval); d > 0 {
		cfg.Checker.PublicRefreshInterval = d
	}
	if d := config.ParseDuration(input.Body.Checker.CheckInterval, cfg.Checker.CheckInterval); d > 0 {
		cfg.Checker.CheckInterval = d
	}
	cfg.Checker.Xray = input.Body.Checker.Xray

	if err := loader.Save(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to save config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to save settings")
	}

	h.logger.Info("settings updated and saved")
	return nil, nil
}
