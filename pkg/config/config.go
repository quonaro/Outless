package config

import (
	"fmt"
	"strings"
	"time"
)

// Config holds unified configuration for all Outless services.
type Config struct {
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Admin    AdminConfig    `yaml:"admin"`
	API      APIConfig      `yaml:"api"`
	Monitor  MonitorConfig  `yaml:"monitor"`
	Router   RouterConfig   `yaml:"router"`
	Logs     LogsConfig     `yaml:"logs"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	URL string `yaml:"url" json:"url"`
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
	DBPath string        `yaml:"db_path" json:"db_path"`
	DBURL  string        `yaml:"db_url" json:"db_url"`
	Auto   bool          `yaml:"auto" json:"auto"`
	TTL    time.Duration `yaml:"ttl" json:"ttl"`
}

// AgentsConfig holds probe agents configuration.
type AgentsConfig struct {
	Workers int    `yaml:"workers" json:"workers"`
	URL     string `yaml:"url" json:"url"`
}

// RouterConfig holds Router (Xray edge) configuration.
type RouterConfig struct {
	Domain       string        `yaml:"domain" json:"Domain"`
	Port         int           `yaml:"port" json:"Port"`
	SNI          string        `yaml:"sni" json:"SNI"`
	PublicKey    string        `yaml:"public_key" json:"PublicKey"`
	PrivateKey   string        `yaml:"private_key" json:"PrivateKey"`
	ShortID      string        `yaml:"short_id" json:"ShortID"`
	Fingerprint  string        `yaml:"fingerprint" json:"Fingerprint"`
	Address      string        `yaml:"address" json:"Address"`
	SyncInterval time.Duration `yaml:"sync_interval" json:"SyncInterval"`
	NameTemplate string        `yaml:"name_template" json:"NameTemplate"`
}

// LogsConfig holds logging configuration.
type LogsConfig struct {
	Level    string `yaml:"level"`
	Colored  bool   `yaml:"colored"`
	Type     string `yaml:"type"`
	FilePath string `yaml:"file_path"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		Database: DatabaseConfig{
			URL: "postgres://outless:outless@localhost:5432/outless?sslmode=disable",
		},
		JWT: JWTConfig{
			Secret: "CHANGE_ME_IN_PRODUCTION",
			Expiry: 24 * time.Hour,
		},
		Admin: AdminConfig{
			Login:    "",
			Password: "",
		},
		API: APIConfig{
			Shutdown: 10 * time.Second,
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
			Domain:       "",
			Port:         443,
			SNI:          "",
			PublicKey:    "",
			PrivateKey:   "",
			ShortID:      "",
			Fingerprint:  "chrome",
			Address:      ":443",
			SyncInterval: 30 * time.Second,
			NameTemplate: "{{vless.country_flag}} {{vless.country}} | {{vless.group}} | {{vless.ping}}ms",
		},
		Logs: LogsConfig{
			Level:    "info",
			Colored:  true,
			Type:     "pretty",
			FilePath: "/var/log/outless/outless.log",
		},
	}
}

// Validate checks critical configuration values and returns an error if they are invalid.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.JWT.Secret) == "CHANGE_ME_IN_PRODUCTION" {
		return fmt.Errorf("JWT secret must be changed from default value")
	}
	if strings.TrimSpace(c.JWT.Secret) == "" {
		return fmt.Errorf("JWT secret cannot be empty")
	}
	if strings.TrimSpace(c.Router.Domain) == "" {
		return fmt.Errorf("router domain cannot be empty")
	}
	return nil
}
