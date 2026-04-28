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
	GeoIP    GeoIPConfig    `yaml:"geoip"`
	Router   RouterConfig   `yaml:"router"`
	XrayAPI  XrayAPIConfig  `yaml:"xray_api"`
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
	Shutdown time.Duration `yaml:"shutdown"`
	JWT      JWTConfig     `yaml:"jwt"`
	Admin    AdminConfig   `yaml:"admin"`
}

// GeoIPConfig controls local MMDB country lookup and optional auto-update.
type GeoIPConfig struct {
	DBPath string        `yaml:"db_path" json:"db_path"`
	DBURL  string        `yaml:"db_url" json:"db_url"`
	Auto   bool          `yaml:"auto" json:"auto"`
	TTL    time.Duration `yaml:"ttl" json:"ttl"`
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

// XrayAPIConfig holds external Xray gRPC API connection settings.
type XrayAPIConfig struct {
	Address string `yaml:"address" json:"address"`
}

// RotationConfig holds log rotation configuration.
type RotationConfig struct {
	MaxSizeMB  int  `yaml:"max_size_mb"`
	MaxBackups int  `yaml:"max_backups"`
	MaxAgeDays int  `yaml:"max_age_days"`
	Compress   bool `yaml:"compress"`
}

// LogsConfig holds logging configuration.
type LogsConfig struct {
	Level       string         `yaml:"level"`
	Colored     bool           `yaml:"colored"`
	Type        string         `yaml:"type"`
	FilePattern string         `yaml:"file_pattern"`
	Rotation    RotationConfig `yaml:"rotation"`
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
		GeoIP: GeoIPConfig{
			DBPath: "",
			DBURL:  "",
			Auto:   false,
			TTL:    24 * time.Hour,
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
			NameTemplate: "{{vless.country_flag}} {{vless.country}} | {{vless.group}}",
		},
		XrayAPI: XrayAPIConfig{
			Address: "127.0.0.1:10085",
		},
		Logs: LogsConfig{
			Level:       "info",
			Colored:     true,
			Type:        "pretty",
			FilePattern: "/var/log/outless/{module}.json",
			Rotation: RotationConfig{
				MaxSizeMB:  100,
				MaxBackups: 10,
				MaxAgeDays: 30,
				Compress:   true,
			},
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
	if strings.TrimSpace(c.XrayAPI.Address) == "" {
		return fmt.Errorf("xray_api address cannot be empty")
	}
	return nil
}
