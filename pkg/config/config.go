package config

import (
	"time"
)

// Config holds unified configuration for all Outless services.
type Config struct {
	Database DatabaseConfig `yaml:"database"`
	API      APIConfig      `yaml:"api"`
	Checker  CheckerConfig  `yaml:"checker"`
	Hub      HubConfig      `yaml:"hub"`
}

// HubConfig holds Hub (Xray relay) configuration.
type HubConfig struct {
	Host          string        `yaml:"host"`
	Port          int           `yaml:"port"`
	SNI           string        `yaml:"sni"`
	PublicKey     string        `yaml:"public_key"`
	PrivateKey    string        `yaml:"private_key"`
	ShortID       string        `yaml:"short_id"`
	Fingerprint   string        `yaml:"fingerprint"`
	ListenAddress string        `yaml:"listen_address"`
	ConfigPath    string        `yaml:"config_path"`
	XrayBinary    string        `yaml:"xray_binary"`
	SyncInterval  time.Duration `yaml:"sync_interval"`
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
	CheckInterval         time.Duration `yaml:"check_interval"`
	Xray                  XrayConfig    `yaml:"xray"`
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
			Workers:               16,
			LatencyFilter:         500 * time.Millisecond,
			PublicRefreshInterval: 10 * time.Minute,
			CheckInterval:         10 * time.Minute,
			Xray: XrayConfig{
				AdminURL:  "http://localhost:10085",
				ProbeURL:  "https://www.google.com/generate_204",
				SocksAddr: "127.0.0.1:1080",
			},
		},
		Hub: HubConfig{
			Host:          "hub.example.com",
			Port:          443,
			SNI:           "www.google.com",
			PublicKey:     "",
			ShortID:       "",
			Fingerprint:   "chrome",
			ListenAddress: ":443",
			ConfigPath:    "/var/lib/outless/xray-hub.json",
			XrayBinary:    "xray",
			SyncInterval:  30 * time.Second,
		},
	}
}
