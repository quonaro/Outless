package config

import (
	"fmt"
	"strings"
	"time"
)

// Config holds unified configuration for all Outless services.
type Config struct {
	Database DatabaseConfig `yaml:"database"`
	API      APIConfig      `yaml:"api"`
	Monitor  MonitorConfig  `yaml:"monitor"`
	Router   RouterConfig   `yaml:"router"`
	Xray     XrayConfig     `yaml:"xray"`
	Logs     LogsConfig     `yaml:"logs"`
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

// APIConfig holds API server configuration.
type APIConfig struct {
	Shutdown time.Duration `yaml:"shutdown_timeout"`
	JWT      JWTConfig     `yaml:"jwt"`
	Admin    AdminConfig   `yaml:"admin"`
}

// MonitorConfig holds monitor (node availability checker) configuration.
type MonitorConfig struct {
	Workers         int           `yaml:"workers"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	PollInterval    time.Duration `yaml:"poll_interval"`
	CheckInterval   time.Duration `yaml:"check_interval"`
	GeoIP           GeoIPConfig   `yaml:"geoip"`
	Agents          AgentsConfig  `yaml:"agents"`
}

// GeoIPConfig controls local MMDB country lookup and optional auto-update.
type GeoIPConfig struct {
	DBPath string        `yaml:"db_path"`
	DBURL  string        `yaml:"db_url"`
	Auto   bool          `yaml:"auto"`
	TTL    time.Duration `yaml:"ttl"`
}

// AgentsConfig holds probe agents configuration.
type AgentsConfig struct {
	Workers int    `yaml:"workers"`
	URL     string `yaml:"url"`
}

// RouterConfig holds Router (Xray edge) configuration.
type RouterConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	SNI          string        `yaml:"sni"`
	PublicKey    string        `yaml:"public_key"`
	PrivateKey   string        `yaml:"private_key"`
	ShortID      string        `yaml:"short_id"`
	Fingerprint  string        `yaml:"fingerprint"`
	Address      string        `yaml:"address"`
	SyncInterval time.Duration `yaml:"sync_interval"`
	ConfigPath   string        `yaml:"config_path"`
	XrayBinary   string        `yaml:"xray_binary"`
}

// XrayConfig holds Xray runtime configuration.
type XrayConfig struct {
	Edge  XrayInstanceConfig `yaml:"edge"`
	Probe XrayInstanceConfig `yaml:"probe"`
}

// XrayInstanceConfig holds configuration for a single Xray instance.
type XrayInstanceConfig struct {
	RuntimeMode string        `yaml:"runtime_mode"`
	AdminURL    string        `yaml:"admin_url"`
	SocksAddr   string        `yaml:"socks_addr"`
	ConfigPath  string        `yaml:"config_path"`
	XrayBinary  string        `yaml:"xray_binary"`
	ProbeURL    string        `yaml:"probe_url,omitempty"`
	ShardCount  int           `yaml:"shard_count,omitempty"`
	Shards      []string      `yaml:"shards,omitempty"`
	GeoIPDBPath string        `yaml:"geoip_db_path,omitempty"`
	GeoIPDBURL  string        `yaml:"geoip_db_url,omitempty"`
	GeoIPAuto   bool          `yaml:"geoip_auto,omitempty"`
	GeoIPTTL    time.Duration `yaml:"geoip_ttl,omitempty"`
}

// LogsConfig holds logging configuration.
type LogsConfig struct {
	Level   string `yaml:"level"`
	Colored bool   `yaml:"colored"`
	Type    string `yaml:"type"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		Database: DatabaseConfig{
			URL: "postgres://outless:outless@localhost:5432/outless?sslmode=disable",
		},
		API: APIConfig{
			Shutdown: 10 * time.Second,
			JWT: JWTConfig{
				Secret: "CHANGE_ME_IN_PRODUCTION",
				Expiry: 24 * time.Hour,
			},
			Admin: AdminConfig{
				Login:    "",
				Password: "",
			},
		},
		Monitor: MonitorConfig{
			Workers:         16,
			RefreshInterval: 10 * time.Minute,
			PollInterval:    5 * time.Second,
			CheckInterval:   10 * time.Minute,
			GeoIP: GeoIPConfig{
				DBPath: "",
				DBURL:  "",
				Auto:   false,
				TTL:    24 * time.Hour,
			},
			Agents: AgentsConfig{
				Workers: 2,
				URL:     "https://www.google.com/generate_204",
			},
		},
		Router: RouterConfig{
			Host:         "localhost",
			Port:         443,
			SNI:          "www.google.com",
			PublicKey:    "",
			PrivateKey:   "",
			ShortID:      "",
			Fingerprint:  "chrome",
			Address:      ":443",
			SyncInterval: 30 * time.Second,
			ConfigPath:   "/var/lib/outless/xray-hub.json",
			XrayBinary:   "xray",
		},
		Xray: XrayConfig{
			Edge: XrayInstanceConfig{
				RuntimeMode: "embedded",
				AdminURL:    "http://127.0.0.1:10086",
				SocksAddr:   "127.0.0.1:1081",
				ConfigPath:  "/var/lib/outless/xray-hub.json",
				XrayBinary:  "xray",
			},
			Probe: XrayInstanceConfig{
				RuntimeMode: "embedded",
				AdminURL:    "http://127.0.0.1:10085",
				ProbeURL:    "https://www.google.com/generate_204",
				SocksAddr:   "127.0.0.1:1080",
				ShardCount:  2,
				Shards:      []string{},
				XrayBinary:  "xray",
				GeoIPDBPath: "/app/tmp/GeoLite2-Country.mmdb",
				GeoIPDBURL:  "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb",
				GeoIPAuto:   true,
				GeoIPTTL:    24 * time.Hour,
			},
		},
		Logs: LogsConfig{
			Level:   "info",
			Colored: true,
			Type:    "pretty",
		},
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
