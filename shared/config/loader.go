package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Loader loads YAML configuration with auto-creation support.
type Loader struct {
	logger *slog.Logger
}

// NewLoader creates a new config loader.
func NewLoader(logger *slog.Logger) *Loader {
	return &Loader{logger: logger}
}

// envVarRegex matches ${VAR} and ${VAR:-default} patterns.
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)}`)

// expandEnv replaces ${VAR} and ${VAR:-default} with environment variable values.
func expandEnv(input string) string {
	return envVarRegex.ReplaceAllStringFunc(input, func(match string) string {
		content := match[2 : len(match)-1] // Remove ${ and }

		// Check for default value syntax: ${VAR:-default}
		if idx := strings.Index(content, ":-"); idx != -1 {
			varName := content[:idx]
			defaultValue := content[idx+2:]
			if val := os.Getenv(varName); val != "" {
				return val
			}
			return defaultValue
		}

		// Simple ${VAR} syntax
		if val := os.Getenv(content); val != "" {
			return val
		}
		return match // Return original if not found
	})
}

// LoadOrCreate loads config from path, creating default if missing.
// If config is created, sensitive fields (like JWT secret) are auto-generated.
func (l *Loader) LoadOrCreate(path string, defaults any) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving config path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		l.logger.Info("config file not found, creating default", slog.String("path", absPath))
		if err := l.createDefault(absPath, defaults); err != nil {
			return fmt.Errorf("creating default config: %w", err)
		}
		l.logger.Info("default config created", slog.String("path", absPath))
	} else if err != nil {
		return fmt.Errorf("checking config file: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	// Expand environment variables before parsing
	expanded := expandEnv(string(data))

	if err := yaml.Unmarshal([]byte(expanded), defaults); err != nil {
		return fmt.Errorf("parsing config YAML: %w", err)
	}

	return nil
}

// createDefault writes default config to file with generated secrets.
func (l *Loader) createDefault(path string, config any) error {
	// Auto-generate secrets for config
	if cfg, ok := config.(*Config); ok {
		if cfg.Auth.JWT.Secret == "CHANGE_ME_IN_PRODUCTION" {
			secret, err := GenerateRandomSecret(32)
			if err != nil {
				return fmt.Errorf("generating JWT secret: %w", err)
			}
			cfg.Auth.JWT.Secret = secret
		}
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling default config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing default config: %w", err)
	}

	return nil
}

// Save writes config to file atomically.
func (l *Loader) Save(path string, config any) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Atomic write: write to temp file then rename
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up on error
		return fmt.Errorf("renaming temp config: %w", err)
	}

	l.logger.Info("config saved", slog.String("path", path))
	return nil
}

// GenerateRandomSecret creates a random hex string for secrets.
func GenerateRandomSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random secret: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// ParseDuration parses duration string with fallback.
func ParseDuration(s string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return fallback
}
