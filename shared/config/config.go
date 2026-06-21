package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"
)

// Config holds unified configuration for the Outless monolith.
type Config struct {
	App      AppConfig `yaml:"app"`
	JWT      JWTConfig `yaml:"jwt"`
	Database Database  `yaml:"database"`
}

// AppConfig holds application-wide settings.
type AppConfig struct {
	ShutdownGracetime time.Duration `yaml:"shutdown_gracetime"`
	HTTPPort          int           `yaml:"http_port"`
	ExternalHost      string        `yaml:"external_host"`     // host used in subscription URLs when inbound.URLHost is empty
	SingboxLogLevel   string        `yaml:"singbox_log_level"` // sing-box log level: trace/debug/info/warn/error/fatal/panic; empty = warn
	LogLevel          string        `yaml:"log_level"`         // process log level: debug/info/warn/error
	DisableDocs       bool          `yaml:"disable_docs"`
}

// Database is the path to the SQLite database file.
type Database string

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret string        `yaml:"secret"`
	Expiry time.Duration `yaml:"expiry"`
}

// DefaultConfig returns default configuration tuned for a single-binary deployment.
func DefaultConfig() Config {
	return Config{
		App: AppConfig{
			ShutdownGracetime: 10 * time.Second,
			HTTPPort:          41220,
			LogLevel:          "info",
		},
		JWT: JWTConfig{
			Secret: "CHANGE_ME_IN_PRODUCTION",
			Expiry: 24 * time.Hour,
		},
		Database: "/var/lib/outless/outless.db",
	}
}

// GenerateRealityKeyPair generates a new x25519 key pair for REALITY.
// The returned strings are base64.RawURLEncoding encoded 32-byte keys.
func GenerateRealityKeyPair() (privateKey string, publicKey string, err error) {
	priv := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(priv); err != nil {
		return "", "", fmt.Errorf("reading random reality private key: %w", err)
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("deriving reality public key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(priv), base64.RawURLEncoding.EncodeToString(pub), nil
}

// DeriveRealityPublicKey derives a REALITY public key from a private key.
func DeriveRealityPublicKey(privateKey string) (string, error) {
	priv := strings.TrimSpace(privateKey)
	if priv == "" {
		return "", fmt.Errorf("private key is empty")
	}
	privBytes, err := base64.RawURLEncoding.DecodeString(priv)
	if err != nil {
		return "", fmt.Errorf("decoding reality private key: %w", err)
	}
	if len(privBytes) != curve25519.ScalarSize {
		return "", fmt.Errorf("invalid reality private key length: %d", len(privBytes))
	}
	pubBytes, err := curve25519.X25519(privBytes, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("deriving reality public key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(pubBytes), nil
}

// GenerateRealityShortID generates a random REALITY short_id (8 bytes, hex encoded).
func GenerateRealityShortID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("reading random short_id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// Validate checks critical configuration values and returns an error if they are invalid.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.JWT.Secret) == "CHANGE_ME_IN_PRODUCTION" {
		return fmt.Errorf("JWT secret must be changed from default value")
	}
	if strings.TrimSpace(c.JWT.Secret) == "" {
		return fmt.Errorf("JWT secret cannot be empty")
	}
	if strings.TrimSpace(string(c.Database)) == "" {
		return fmt.Errorf("database path cannot be empty")
	}
	return nil
}
