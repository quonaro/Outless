package config

import (
	"fmt"
	"strings"
	"time"
)

// Config holds unified configuration for all Outless services.
type Config struct {
	Database DatabaseConfig  `yaml:"database"`
	API      APIConfig       `yaml:"api"`
	Checker  CheckerConfig   `yaml:"checker"`
	Hub      HubConfig       `yaml:"hub"`
	Xray     XrayRolesConfig `yaml:"xray"`
}

// HubConfig holds Hub (Xray relay) configuration.
type HubConfig struct {
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	SNI           string `yaml:"sni"`
	PublicKey     string `yaml:"public_key"`
	PrivateKey    string `yaml:"private_key"`
	ShortID       string `yaml:"short_id"`
	Fingerprint   string `yaml:"fingerprint"`
	ListenAddress string `yaml:"listen_address"`
	// ConfigPath is a legacy compatibility field.
	ConfigPath string `yaml:"config_path"`
	// XrayBinary is a legacy compatibility field.
	XrayBinary   string        `yaml:"xray_binary"`
	SyncInterval time.Duration `yaml:"sync_interval"`
}

// APIConfig holds API server configuration.
type APIConfig struct {
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	JWT             JWTConfig     `yaml:"jwt"`
	Admin           AdminConfig   `yaml:"admin"`
}

// CheckerConfig holds health checker configuration.
type CheckerConfig struct {
	Workers               int           `yaml:"workers"`
	LatencyFilter         time.Duration `yaml:"latency_filter"`
	PublicRefreshInterval time.Duration `yaml:"public_refresh_interval"`
	JobPollInterval       time.Duration `yaml:"job_poll_interval"`
	CheckInterval         time.Duration `yaml:"check_interval"`
	// Xray is a legacy compatibility field.
	Xray XrayConfig `yaml:"xray"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	URL string `yaml:"url"`
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret string        `yaml:"secret"`
	Expiry time.Duration `yaml:"expiry"`
}

// AdminConfig holds admin bootstrap settings.
type AdminConfig struct {
	Login    string `yaml:"login"`
	Password string `yaml:"password"`
}

// XrayConfig holds Xray engine settings.
type XrayConfig struct {
	AdminURL string `yaml:"admin_url"`
	ProbeURL string `yaml:"probe_url"`
	// SocksAddr is the host:port of the local SOCKS inbound used to run HTTP probes through Xray (e.g. 127.0.0.1:1080).
	SocksAddr string `yaml:"socks_addr"`
	// GeoIPDBPath points to a local MMDB file for offline country lookup (e.g. /app/GeoLite2-Country.mmdb).
	GeoIPDBPath string `yaml:"geoip_db_path"`
	// GeoIPDBURL is an optional URL for downloading MMDB when auto-update is enabled.
	GeoIPDBURL string `yaml:"geoip_db_url"`
	// GeoIPAuto enables periodic TTL-based auto-refresh of MMDB from GeoIPDBURL.
	GeoIPAuto bool `yaml:"geoip_auto"`
	// GeoIPTTL defines refresh interval for auto-update.
	GeoIPTTL time.Duration `yaml:"geoip_ttl"`
}

// XrayRuntimeMode defines how hub manages the edge Xray process.
type XrayRuntimeMode string

const (
	// XrayRuntimeEmbedded means hub starts/stops local Xray process itself.
	XrayRuntimeEmbedded XrayRuntimeMode = "embedded"
	// XrayRuntimeExternal means hub only writes config and expects external Xray lifecycle management.
	XrayRuntimeExternal XrayRuntimeMode = "external"
)

const (
	defaultEdgeAdminURL   = "http://localhost:10086"
	defaultEdgeSocksAddr  = "127.0.0.1:1081"
	defaultEdgeConfigPath = "/var/lib/outless/xray-hub.json"
	defaultEdgeBinary     = "xray"

	defaultProbeAdminURL = "http://localhost:10085"
	defaultProbeURL      = "https://www.google.com/generate_204"
	defaultProbeSocks    = "127.0.0.1:1080"
)

// XrayRolesConfig defines role-separated Xray runtime settings.
type XrayRolesConfig struct {
	Edge  XrayEdgeConfig  `yaml:"edge"`
	Probe XrayProbeConfig `yaml:"probe"`
}

// XrayEdgeConfig holds settings for the relay (hub-facing) Xray instance.
type XrayEdgeConfig struct {
	AdminURL    string          `yaml:"admin_url"`
	SocksAddr   string          `yaml:"socks_addr"`
	ConfigPath  string          `yaml:"config_path"`
	XrayBinary  string          `yaml:"xray_binary"`
	RuntimeMode XrayRuntimeMode `yaml:"runtime_mode"`
}

// XrayProbeConfig holds settings for checker/API probe Xray instance.
type XrayProbeConfig struct {
	AdminURL string                 `yaml:"admin_url"`
	ProbeURL string                 `yaml:"probe_url"`
	Shards   []XrayProbeShardConfig `yaml:"shards"`
	// ShardCount specifies the number of probe shards to create dynamically.
	// If set, shards are auto-generated as xray-probe-1, xray-probe-2, etc.
	// Takes precedence over explicit Shards array when both are present.
	ShardCount int `yaml:"shard_count"`
	// RuntimeMode defines how probe Xray processes are managed (embedded or docker).
	RuntimeMode XrayRuntimeMode `yaml:"runtime_mode"`
	// XrayBinary is the path to the Xray binary for embedded mode.
	XrayBinary string `yaml:"xray_binary"`
	// SocksAddr is the host:port of the local SOCKS inbound used to run HTTP probes through Xray (e.g. 127.0.0.1:1080).
	SocksAddr string `yaml:"socks_addr"`
	// GeoIPDBPath points to a local MMDB file for offline country lookup (e.g. /app/GeoLite2-Country.mmdb).
	GeoIPDBPath string `yaml:"geoip_db_path"`
	// GeoIPDBURL is an optional URL for downloading MMDB when auto-update is enabled.
	GeoIPDBURL string `yaml:"geoip_db_url"`
	// GeoIPAuto enables periodic TTL-based auto-refresh of MMDB from GeoIPDBURL.
	GeoIPAuto bool `yaml:"geoip_auto"`
	// GeoIPTTL defines refresh interval for auto-update.
	GeoIPTTL time.Duration `yaml:"geoip_ttl"`
}

// XrayProbeShardConfig defines one independent probe runtime channel.
type XrayProbeShardConfig struct {
	AdminURL  string `yaml:"admin_url"`
	SocksAddr string `yaml:"socks_addr"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	cfg := Config{
		Database: DatabaseConfig{
			URL: "postgres://outless:outless@localhost:5432/outless?sslmode=disable",
		},
		API: APIConfig{
			ShutdownTimeout: 10 * time.Second,
			JWT: JWTConfig{
				Secret: "CHANGE_ME_IN_PRODUCTION",
				Expiry: 24 * time.Hour,
			},
			Admin: AdminConfig{
				Login:    "",
				Password: "",
			},
		},
		Checker: CheckerConfig{
			Workers:               16,
			LatencyFilter:         500 * time.Millisecond,
			PublicRefreshInterval: 10 * time.Minute,
			JobPollInterval:       5 * time.Second,
			CheckInterval:         10 * time.Minute,
			Xray: XrayConfig{
				AdminURL:    "http://localhost:10085",
				ProbeURL:    "https://www.google.com/generate_204",
				SocksAddr:   "127.0.0.1:1080",
				GeoIPDBPath: "",
				GeoIPDBURL:  "",
				GeoIPAuto:   false,
				GeoIPTTL:    24 * time.Hour,
			},
		},
		Hub: HubConfig{
			Host:          "localhost",
			Port:          443,
			SNI:           "www.google.com",
			PublicKey:     "",
			PrivateKey:    "",
			ShortID:       "",
			Fingerprint:   "chrome",
			ListenAddress: ":443",
			ConfigPath:    "/var/lib/outless/xray-hub.json",
			XrayBinary:    "xray",
			SyncInterval:  30 * time.Second,
		},
		Xray: XrayRolesConfig{
			Edge: XrayEdgeConfig{
				AdminURL:    defaultEdgeAdminURL,
				SocksAddr:   defaultEdgeSocksAddr,
				ConfigPath:  defaultEdgeConfigPath,
				XrayBinary:  defaultEdgeBinary,
				RuntimeMode: XrayRuntimeEmbedded,
			},
			Probe: XrayProbeConfig{
				AdminURL:    defaultProbeAdminURL,
				ProbeURL:    defaultProbeURL,
				SocksAddr:   defaultProbeSocks,
				GeoIPDBPath: "",
				GeoIPDBURL:  "",
				GeoIPAuto:   false,
				GeoIPTTL:    24 * time.Hour,
			},
		},
	}
	cfg.ApplyCompatibility()
	return cfg
}

// generateShardsFromCount creates shard configs for xray-probe-1, xray-probe-2, etc.
func generateShardsFromCount(count int, _, _ string) []XrayProbeShardConfig {
	if count <= 0 {
		return nil
	}
	shards := make([]XrayProbeShardConfig, count)
	for i := 0; i < count; i++ {
		shardNum := i + 1
		shards[i] = XrayProbeShardConfig{
			AdminURL:  fmt.Sprintf("http://xray-probe-%d:10085", shardNum),
			SocksAddr: fmt.Sprintf("xray-probe-%d:1080", shardNum),
		}
	}
	return shards
}

// ApplyCompatibility maps legacy xray fields into role-based config and backfills defaults.
func (c *Config) ApplyCompatibility() {
	// Hub (legacy) -> xray.edge
	if strings.TrimSpace(c.Hub.ConfigPath) != "" &&
		(strings.TrimSpace(c.Xray.Edge.ConfigPath) == "" || c.Xray.Edge.ConfigPath == defaultEdgeConfigPath) {
		c.Xray.Edge.ConfigPath = c.Hub.ConfigPath
	}
	if strings.TrimSpace(c.Hub.XrayBinary) != "" &&
		(strings.TrimSpace(c.Xray.Edge.XrayBinary) == "" || c.Xray.Edge.XrayBinary == defaultEdgeBinary) {
		c.Xray.Edge.XrayBinary = c.Hub.XrayBinary
	}

	// Checker (legacy) -> xray.probe
	if strings.TrimSpace(c.Checker.Xray.AdminURL) != "" &&
		(strings.TrimSpace(c.Xray.Probe.AdminURL) == "" || c.Xray.Probe.AdminURL == defaultProbeAdminURL) {
		c.Xray.Probe.AdminURL = c.Checker.Xray.AdminURL
	}
	if strings.TrimSpace(c.Checker.Xray.ProbeURL) != "" &&
		(strings.TrimSpace(c.Xray.Probe.ProbeURL) == "" || c.Xray.Probe.ProbeURL == defaultProbeURL) {
		c.Xray.Probe.ProbeURL = c.Checker.Xray.ProbeURL
	}
	if strings.TrimSpace(c.Checker.Xray.SocksAddr) != "" &&
		(strings.TrimSpace(c.Xray.Probe.SocksAddr) == "" || c.Xray.Probe.SocksAddr == defaultProbeSocks) {
		c.Xray.Probe.SocksAddr = c.Checker.Xray.SocksAddr
	}
	if strings.TrimSpace(c.Xray.Probe.GeoIPDBPath) == "" && strings.TrimSpace(c.Checker.Xray.GeoIPDBPath) != "" {
		c.Xray.Probe.GeoIPDBPath = c.Checker.Xray.GeoIPDBPath
	}
	if strings.TrimSpace(c.Xray.Probe.GeoIPDBURL) == "" && strings.TrimSpace(c.Checker.Xray.GeoIPDBURL) != "" {
		c.Xray.Probe.GeoIPDBURL = c.Checker.Xray.GeoIPDBURL
	}
	if c.Checker.Xray.GeoIPTTL > 0 &&
		(c.Xray.Probe.GeoIPTTL <= 0 || c.Xray.Probe.GeoIPTTL == 24*time.Hour) {
		c.Xray.Probe.GeoIPTTL = c.Checker.Xray.GeoIPTTL
	}
	if !c.Xray.Probe.GeoIPAuto && c.Checker.Xray.GeoIPAuto {
		c.Xray.Probe.GeoIPAuto = c.Checker.Xray.GeoIPAuto
	}

	// Defaults for xray.edge
	if strings.TrimSpace(c.Xray.Edge.AdminURL) == "" {
		c.Xray.Edge.AdminURL = defaultEdgeAdminURL
	}
	if strings.TrimSpace(c.Xray.Edge.SocksAddr) == "" {
		c.Xray.Edge.SocksAddr = defaultEdgeSocksAddr
	}
	if strings.TrimSpace(c.Xray.Edge.ConfigPath) == "" {
		c.Xray.Edge.ConfigPath = defaultEdgeConfigPath
	}
	if strings.TrimSpace(c.Xray.Edge.XrayBinary) == "" {
		c.Xray.Edge.XrayBinary = defaultEdgeBinary
	}
	if c.Xray.Edge.RuntimeMode == "" {
		c.Xray.Edge.RuntimeMode = XrayRuntimeEmbedded
	}

	// Defaults for xray.probe
	if strings.TrimSpace(c.Xray.Probe.AdminURL) == "" {
		c.Xray.Probe.AdminURL = defaultProbeAdminURL
	}
	if strings.TrimSpace(c.Xray.Probe.ProbeURL) == "" {
		c.Xray.Probe.ProbeURL = defaultProbeURL
	}
	if strings.TrimSpace(c.Xray.Probe.SocksAddr) == "" {
		c.Xray.Probe.SocksAddr = defaultProbeSocks
	}
	if c.Xray.Probe.RuntimeMode == "" {
		c.Xray.Probe.RuntimeMode = XrayRuntimeEmbedded
	}
	if strings.TrimSpace(c.Xray.Probe.XrayBinary) == "" {
		c.Xray.Probe.XrayBinary = defaultEdgeBinary
	}
	if c.Xray.Probe.GeoIPTTL <= 0 {
		c.Xray.Probe.GeoIPTTL = 24 * time.Hour
	}
	// Generate shards from shard_count if specified
	if c.Xray.Probe.ShardCount > 0 {
		c.Xray.Probe.Shards = generateShardsFromCount(c.Xray.Probe.ShardCount, c.Xray.Probe.AdminURL, c.Xray.Probe.SocksAddr)
	} else if len(c.Xray.Probe.Shards) == 0 {
		c.Xray.Probe.Shards = []XrayProbeShardConfig{
			{
				AdminURL:  c.Xray.Probe.AdminURL,
				SocksAddr: c.Xray.Probe.SocksAddr,
			},
		}
	} else {
		for i := range c.Xray.Probe.Shards {
			if strings.TrimSpace(c.Xray.Probe.Shards[i].AdminURL) == "" {
				c.Xray.Probe.Shards[i].AdminURL = c.Xray.Probe.AdminURL
			}
			if strings.TrimSpace(c.Xray.Probe.Shards[i].SocksAddr) == "" {
				c.Xray.Probe.Shards[i].SocksAddr = c.Xray.Probe.SocksAddr
			}
		}
	}
	if strings.TrimSpace(c.Xray.Probe.AdminURL) == "" && len(c.Xray.Probe.Shards) > 0 {
		c.Xray.Probe.AdminURL = c.Xray.Probe.Shards[0].AdminURL
	}
	if strings.TrimSpace(c.Xray.Probe.SocksAddr) == "" && len(c.Xray.Probe.Shards) > 0 {
		c.Xray.Probe.SocksAddr = c.Xray.Probe.Shards[0].SocksAddr
	}

	// Backfill legacy fields for transitional compatibility.
	if strings.TrimSpace(c.Hub.ConfigPath) == "" {
		c.Hub.ConfigPath = c.Xray.Edge.ConfigPath
	}
	if strings.TrimSpace(c.Hub.XrayBinary) == "" {
		c.Hub.XrayBinary = c.Xray.Edge.XrayBinary
	}
	if strings.TrimSpace(c.Checker.Xray.AdminURL) == "" {
		c.Checker.Xray.AdminURL = c.Xray.Probe.AdminURL
	}
	if strings.TrimSpace(c.Checker.Xray.ProbeURL) == "" {
		c.Checker.Xray.ProbeURL = c.Xray.Probe.ProbeURL
	}
	if strings.TrimSpace(c.Checker.Xray.SocksAddr) == "" {
		c.Checker.Xray.SocksAddr = c.Xray.Probe.SocksAddr
	}
	if strings.TrimSpace(c.Checker.Xray.GeoIPDBPath) == "" {
		c.Checker.Xray.GeoIPDBPath = c.Xray.Probe.GeoIPDBPath
	}
	if strings.TrimSpace(c.Checker.Xray.GeoIPDBURL) == "" {
		c.Checker.Xray.GeoIPDBURL = c.Xray.Probe.GeoIPDBURL
	}
	if !c.Checker.Xray.GeoIPAuto {
		c.Checker.Xray.GeoIPAuto = c.Xray.Probe.GeoIPAuto
	}
	if c.Checker.Xray.GeoIPTTL <= 0 {
		c.Checker.Xray.GeoIPTTL = c.Xray.Probe.GeoIPTTL
	}
}

// Validate checks critical configuration values and returns an error if they are invalid.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.API.JWT.Secret) == "CHANGE_ME_IN_PRODUCTION" {
		return fmt.Errorf("JWT secret must be changed from default value")
	}
	if strings.TrimSpace(c.API.JWT.Secret) == "" {
		return fmt.Errorf("JWT secret cannot be empty")
	}
	return nil
}
