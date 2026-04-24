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
	Shutdown string `json:"shutdown"`
}

// SafeMonitorConfig exposes monitor settings without secrets.
type SafeMonitorConfig struct {
	Workers         int                 `json:"workers"`
	RefreshInterval string              `json:"refresh_interval"`
	PollInterval    string              `json:"poll_interval"`
	CheckInterval   string              `json:"check_interval"`
	GeoIP           config.GeoIPConfig  `json:"geoip"`
	Agents          config.AgentsConfig `json:"agents"`
}

// SettingsOutput is returned by GET /v1/settings.
type SettingsOutput struct {
	Body struct {
		Database config.DatabaseConfig `json:"database"`
		API      config.APIConfig      `json:"api"`
		Monitor  SafeMonitorConfig     `json:"monitor"`
		Router   config.RouterConfig   `json:"router"`
	}
}

// UpdateSettingsInput is accepted by PUT /v1/settings.
type UpdateSettingsInput struct {
	Body struct {
		Database config.DatabaseConfig `json:"database"`
		API      config.APIConfig      `json:"api"`
		Monitor  SafeMonitorConfig     `json:"monitor"`
		Router   config.RouterConfig   `json:"router"`
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
	out.Body.API = cfg.API
	out.Body.Monitor = SafeMonitorConfig{
		Workers:         cfg.Monitor.Workers,
		RefreshInterval: cfg.Monitor.RefreshInterval.String(),
		PollInterval:    cfg.Monitor.PollInterval.String(),
		CheckInterval:   cfg.Monitor.CheckInterval.String(),
		GeoIP:           cfg.Monitor.GeoIP,
		Agents:          cfg.Monitor.Agents,
	}
	out.Body.Router = cfg.Router

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
	cfg.API = input.Body.API

	cfg.Monitor.Workers = input.Body.Monitor.Workers
	if d := config.ParseDuration(input.Body.Monitor.RefreshInterval, cfg.Monitor.RefreshInterval); d > 0 {
		cfg.Monitor.RefreshInterval = d
	}
	if d := config.ParseDuration(input.Body.Monitor.PollInterval, cfg.Monitor.PollInterval); d > 0 {
		cfg.Monitor.PollInterval = d
	}
	if d := config.ParseDuration(input.Body.Monitor.CheckInterval, cfg.Monitor.CheckInterval); d > 0 {
		cfg.Monitor.CheckInterval = d
	}
	cfg.Monitor.GeoIP = input.Body.Monitor.GeoIP
	cfg.Monitor.Agents = input.Body.Monitor.Agents

	cfg.Router = input.Body.Router

	if err := loader.Save(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to save config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to save settings")
	}

	h.logger.Info("settings updated and saved")
	return nil, nil
}
