package config

import (
	"time"
)

// Config holds unified configuration for all Outless services.
type Config struct {
	Database DatabaseConfig `yaml:"database"`
	API      APIConfig      `yaml:"api"`
	Checker  CheckerConfig  `yaml:"checker"`
}

// APIConfig holds API server configuration.
type APIConfig struct {
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	JWT             JWTConfig     `yaml:"jwt"`
	Admin           AdminConfig   `yaml:"admin"`
}

// CheckerConfig holds health checker configuration.
type CheckerConfig struct {
	Workers int        `yaml:"workers"`
	Xray    XrayConfig `yaml:"xray"`
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
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
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
			Workers: 16,
			Xray: XrayConfig{
				AdminURL: "http://xray:10085",
				ProbeURL: "https://www.google.com/generate_204",
			},
		},
	}
}
