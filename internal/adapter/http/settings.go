package http

import (
	"context"
	"log/slog"

	"outless/shared/config"

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

// JWT secret and admin credentials are intentionally omitted.
type SafeAPIConfig struct {
	Shutdown string `json:"shutdown"`
}

// SafeAppConfig exposes app settings.
type SafeAppConfig struct {
	ShutdownGracetime string         `json:"shutdown_gracetime"`
	Logs              SafeLogsConfig `json:"logs"`
}

// SafeAuthConfig exposes auth settings without secrets.
type SafeAuthConfig struct {
	Admin SafeAdminConfig `json:"admin"`
}

// SafeAdminConfig exposes admin settings without secrets.
type SafeAdminConfig struct {
	Login string `json:"login"`
}

// SafeLogsConfig exposes logs settings.
type SafeLogsConfig struct {
	Level   string `json:"level"`
	Colored bool   `json:"colored"`
	Type    string `json:"type"`
	Access  string `json:"access"`
	Error   string `json:"error"`
}

// SafeGeoIPConfig exposes GeoIP settings with expiry as string.
type SafeGeoIPConfig struct {
	DBPath string `json:"db_path"`
	DBURL  string `json:"db_url"`
	Auto   bool   `json:"auto"`
	Expiry string `json:"expiry"`
}

// SafeRouterConfig exposes router settings without secrets.
type SafeRouterConfig struct {
	URLHost      string                  `json:"URLHost"`
	Inbound      SafeRouterInboundConfig `json:"Inbound"`
	API          string                  `json:"API"`
	SyncInterval string                  `json:"SyncInterval"`
	NameTemplate string                  `json:"NameTemplate"`
}

// SafeRouterInboundConfig exposes router inbound settings.
type SafeRouterInboundConfig struct {
	Port        int    `json:"Port"`
	SNI         string `json:"SNI"`
	PublicKey   string `json:"PublicKey"`
	ShortID     string `json:"ShortID"`
	Fingerprint string `json:"Fingerprint"`
}

// SettingsOutput is returned by GET /v1/settings.
type SettingsOutput struct {
	Body struct {
		Database config.Database  `json:"database"`
		App      SafeAppConfig    `json:"app"`
		Auth     SafeAuthConfig   `json:"auth"`
		GeoIP    SafeGeoIPConfig  `json:"geoip"`
		Router   SafeRouterConfig `json:"router"`
	}
}

// UpdateSettingsInput is accepted by PUT /v1/settings.
type UpdateSettingsInput struct {
	Body struct {
		Database config.Database  `json:"database"`
		App      SafeAppConfig    `json:"app"`
		Auth     SafeAuthConfig   `json:"auth"`
		GeoIP    SafeGeoIPConfig  `json:"geoip"`
		Router   SafeRouterConfig `json:"router"`
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
	out.Body.App = SafeAppConfig{
		ShutdownGracetime: cfg.App.ShutdownGracetime.String(),
		Logs: SafeLogsConfig{
			Level:   cfg.App.Logs.Level,
			Colored: cfg.App.Logs.Colored,
			Type:    cfg.App.Logs.Type,
			Access:  cfg.App.Logs.Access,
			Error:   cfg.App.Logs.Error,
		},
	}
	out.Body.Auth = SafeAuthConfig{
		Admin: SafeAdminConfig{
			Login: cfg.Auth.Admin.Login,
		},
	}
	out.Body.GeoIP = SafeGeoIPConfig{
		DBPath: cfg.GeoIP.DBPath,
		DBURL:  cfg.GeoIP.DBURL,
		Auto:   cfg.GeoIP.Auto,
		Expiry: cfg.GeoIP.Expiry.String(),
	}
	out.Body.Router = SafeRouterConfig{
		URLHost: cfg.Router.URLHost,
		Inbound: SafeRouterInboundConfig{
			Port:        cfg.Router.Inbound.Port,
			SNI:         cfg.Router.Inbound.SNI,
			PublicKey:   cfg.Router.Inbound.PublicKey,
			ShortID:     cfg.Router.Inbound.ShortID,
			Fingerprint: cfg.Router.Inbound.Fingerprint,
		},
		API:          cfg.Router.API,
		SyncInterval: cfg.Router.SyncInterval.String(),
		NameTemplate: cfg.Router.NameTemplate,
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
	if d := config.ParseDuration(input.Body.App.ShutdownGracetime, cfg.App.ShutdownGracetime); d > 0 {
		cfg.App.ShutdownGracetime = d
	}
	cfg.App.Logs = config.LogsConfig{
		Level:   input.Body.App.Logs.Level,
		Colored: input.Body.App.Logs.Colored,
		Type:    input.Body.App.Logs.Type,
		Access:  input.Body.App.Logs.Access,
		Error:   input.Body.App.Logs.Error,
	}
	cfg.Auth.Admin.Login = input.Body.Auth.Admin.Login

	cfg.GeoIP = config.GeoIPConfig{
		DBPath: input.Body.GeoIP.DBPath,
		DBURL:  input.Body.GeoIP.DBURL,
		Auto:   input.Body.GeoIP.Auto,
		Expiry: config.ParseDuration(input.Body.GeoIP.Expiry, cfg.GeoIP.Expiry),
	}

	cfg.Router = config.RouterConfig{
		URLHost: input.Body.Router.URLHost,
		Inbound: config.RouterInboundConfig{
			Port:        input.Body.Router.Inbound.Port,
			Address:     "", // Use default 0.0.0.0 in Xray adapter
			SNI:         input.Body.Router.Inbound.SNI,
			PublicKey:   input.Body.Router.Inbound.PublicKey,
			PrivateKey:  cfg.Router.Inbound.PrivateKey,
			ShortID:     input.Body.Router.Inbound.ShortID,
			Fingerprint: input.Body.Router.Inbound.Fingerprint,
		},
		API:          input.Body.Router.API,
		SyncInterval: config.ParseDuration(input.Body.Router.SyncInterval, cfg.Router.SyncInterval),
		NameTemplate: input.Body.Router.NameTemplate,
	}

	if err := loader.Save(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to save config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to save settings")
	}

	h.logger.Info("settings updated and saved")
	return nil, nil
}
