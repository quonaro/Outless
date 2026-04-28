package config

import (
	"fmt"
	"strings"
	"time"
)

// Config holds unified configuration for all Outless services.
type Config struct {
	App      AppConfig    `yaml:"app"`
	Auth     AuthConfig   `yaml:"auth"`
	Database Database     `yaml:"database"`
	GeoIP    GeoIPConfig  `yaml:"geoip"`
	Router   RouterConfig `yaml:"router"`
}

// AppConfig holds application-wide settings.
type AppConfig struct {
	ShutdownGracetime time.Duration `yaml:"shutdown_gracetime"`
	HTTPPort          int           `yaml:"http_port"`
	Logs              LogsConfig    `yaml:"logs"`
	DisableDocs       bool          `yaml:"disable_docs"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Admin AdminConfig `yaml:"admin"`
	JWT   JWTConfig   `yaml:"jwt"`
}

// Database is the database connection DSN string.
type Database string

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

// GeoIPConfig controls local MMDB country lookup and optional auto-update.
type GeoIPConfig struct {
	DBPath string        `yaml:"db_path" json:"db_path"`
	DBURL  string        `yaml:"db_url" json:"db_url"`
	Auto   bool          `yaml:"auto" json:"auto"`
	Expiry time.Duration `yaml:"expiry" json:"expiry"`
}

// RouterConfig holds Router (Xray edge) configuration.
type RouterConfig struct {
	URLHost      string              `yaml:"url_host" json:"URLHost"`
	Inbound      RouterInboundConfig `yaml:"inbound" json:"Inbound"`
	API          string              `yaml:"api" json:"API"`
	SyncInterval time.Duration       `yaml:"sync_interval" json:"SyncInterval"`
	NameTemplate string              `yaml:"name_template" json:"NameTemplate"`
}

// RouterInboundConfig holds Xray inbound (REALITY) configuration.
type RouterInboundConfig struct {
	Port        int    `yaml:"port" json:"Port"`
	Address     string `yaml:"address,omitempty" json:"Address,omitempty"`
	SNI         string `yaml:"sni" json:"SNI"`
	PublicKey   string `yaml:"public_key" json:"PublicKey"`
	PrivateKey  string `yaml:"private_key" json:"PrivateKey"`
	ShortID     string `yaml:"short_id" json:"ShortID"`
	Fingerprint string `yaml:"fingerprint" json:"Fingerprint"`
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
	Level   string `yaml:"level"`
	Colored bool   `yaml:"colored"`
	Type    string `yaml:"type"`
	Output  string `yaml:"output"` // stdout, stderr, none, or file path
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		App: AppConfig{
			ShutdownGracetime: 10 * time.Second,
			HTTPPort:          41220,
			Logs: LogsConfig{
				Level:   "info",
				Colored: true,
				Type:    "pretty",
				Output:  "stdout",
			},
		},
		Auth: AuthConfig{
			Admin: AdminConfig{
				Login:    "",
				Password: "",
			},
			JWT: JWTConfig{
				Secret: "CHANGE_ME_IN_PRODUCTION",
				Expiry: 24 * time.Hour,
			},
		},
		Database: "postgres://outless:outless@localhost:5432/outless?sslmode=disable",
		GeoIP: GeoIPConfig{
			DBPath: "",
			DBURL:  "",
			Auto:   false,
			Expiry: 24 * time.Hour,
		},
		Router: RouterConfig{
			URLHost: "",
			Inbound: RouterInboundConfig{
				Port:        443,
				Address:     ":443",
				SNI:         "",
				PublicKey:   "",
				PrivateKey:  "",
				ShortID:     "",
				Fingerprint: "chrome",
			},
			API:          "127.0.0.1:10085",
			SyncInterval: 30 * time.Second,
			NameTemplate: "{{vless.country_flag}} {{vless.country}} | {{vless.group}}",
		},
	}
}

// Validate checks critical configuration values and returns an error if they are invalid.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Auth.JWT.Secret) == "CHANGE_ME_IN_PRODUCTION" {
		return fmt.Errorf("JWT secret must be changed from default value")
	}
	if strings.TrimSpace(c.Auth.JWT.Secret) == "" {
		return fmt.Errorf("JWT secret cannot be empty")
	}
	if strings.TrimSpace(c.Router.URLHost) == "" {
		return fmt.Errorf("router url_host cannot be empty")
	}
	if strings.TrimSpace(c.Router.API) == "" {
		return fmt.Errorf("router api cannot be empty")
	}
	return nil
}
