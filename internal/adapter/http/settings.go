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

// SafeAPIConfig exposes only non-sensitive API settings.
// JWT secret and admin credentials are intentionally omitted.
type SafeAPIConfig struct {
	Shutdown string `json:"shutdown"`
}

// SafeGeoIPConfig exposes GeoIP settings with TTL as string.
type SafeGeoIPConfig struct {
	DBPath string `json:"db_path"`
	DBURL  string `json:"db_url"`
	Auto   bool   `json:"auto"`
	TTL    string `json:"ttl"`
}

// SafeRouterConfig exposes Router settings with duration fields as strings.
type SafeRouterConfig struct {
	Domain       string `json:"Domain"`
	Port         int    `json:"Port"`
	SNI          string `json:"SNI"`
	PublicKey    string `json:"PublicKey"`
	PrivateKey   string `json:"PrivateKey"`
	ShortID      string `json:"ShortID"`
	Fingerprint  string `json:"Fingerprint"`
	Address      string `json:"Address"`
	SyncInterval string `json:"SyncInterval"`
}

// SettingsOutput is returned by GET /v1/settings.
type SettingsOutput struct {
	Body struct {
		Database config.DatabaseConfig `json:"database"`
		API      SafeAPIConfig         `json:"api"`
		GeoIP    SafeGeoIPConfig       `json:"geoip"`
		Router   SafeRouterConfig      `json:"router"`
	}
}

// UpdateSettingsInput is accepted by PUT /v1/settings.
type UpdateSettingsInput struct {
	Body struct {
		Database config.DatabaseConfig `json:"database"`
		API      SafeAPIConfig         `json:"api"`
		GeoIP    SafeGeoIPConfig       `json:"geoip"`
		Router   SafeRouterConfig      `json:"router"`
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
		Shutdown: cfg.API.Shutdown.String(),
	}
	out.Body.GeoIP = SafeGeoIPConfig{
		DBPath: cfg.GeoIP.DBPath,
		DBURL:  cfg.GeoIP.DBURL,
		Auto:   cfg.GeoIP.Auto,
		TTL:    cfg.GeoIP.TTL.String(),
	}
	out.Body.Router = SafeRouterConfig{
		Domain:       cfg.Router.Domain,
		Port:         cfg.Router.Port,
		SNI:          cfg.Router.SNI,
		PublicKey:    cfg.Router.PublicKey,
		PrivateKey:   cfg.Router.PrivateKey,
		ShortID:      cfg.Router.ShortID,
		Fingerprint:  cfg.Router.Fingerprint,
		Address:      cfg.Router.Address,
		SyncInterval: cfg.Router.SyncInterval.String(),
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
	if d := config.ParseDuration(input.Body.API.Shutdown, cfg.API.Shutdown); d > 0 {
		cfg.API.Shutdown = d
	}

	cfg.GeoIP = config.GeoIPConfig{
		DBPath: input.Body.GeoIP.DBPath,
		DBURL:  input.Body.GeoIP.DBURL,
		Auto:   input.Body.GeoIP.Auto,
		TTL:    config.ParseDuration(input.Body.GeoIP.TTL, cfg.GeoIP.TTL),
	}

	cfg.Router = config.RouterConfig{
		Domain:       input.Body.Router.Domain,
		Port:         input.Body.Router.Port,
		SNI:          input.Body.Router.SNI,
		PublicKey:    input.Body.Router.PublicKey,
		PrivateKey:   input.Body.Router.PrivateKey,
		ShortID:      input.Body.Router.ShortID,
		Fingerprint:  input.Body.Router.Fingerprint,
		Address:      input.Body.Router.Address,
		SyncInterval: config.ParseDuration(input.Body.Router.SyncInterval, cfg.Router.SyncInterval),
	}

	if err := loader.Save(h.configPath, &cfg); err != nil {
		h.logger.Error("failed to save config", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to save settings")
	}

	h.logger.Info("settings updated and saved")
	return nil, nil
}
