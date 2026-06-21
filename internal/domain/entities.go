package domain

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// Node represents a proxy endpoint (exit VLESS server) managed by Outless.
type Node struct {
	ID      string
	URL     string
	GroupID string
	Country string
}

// Token describes subscription access token metadata.
// UUID is the per-token identifier used as a VLESS user id on the hub inbound.
type Token struct {
	ID         string
	Owner      string
	GroupID    string
	GroupIDs   []string
	InboundIDs []string
	UUID       string
	IsActive   bool
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// Group represents a collection of nodes and tokens for access control.
type Group struct {
	ID            string
	Name          string
	SourceURL     string
	TotalNodes    int
	RandomEnabled bool
	RandomLimit   *int
	LastSyncedAt  *time.Time
	CreatedAt     time.Time
}

// PublicSource represents an external source of VLESS nodes.
type PublicSource struct {
	ID            string
	URL           string
	GroupID       string
	LastFetchedAt *time.Time
	CreatedAt     time.Time
}

// Inbound represents a VLESS REALITY entry point managed by Outless.
type Inbound struct {
	ID           string
	Name         string
	Address      string
	Port         int
	SNI          string
	Handshake    string
	PublicKey    string
	PrivateKey   string
	ShortID      string
	Fingerprint  string
	URLHost      string
	NameTemplate string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Admin represents an administrative user with access to management endpoints.
type Admin struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// GenerateGroupID creates a unique group ID.
func GenerateGroupID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating group id: %w", err)
	}
	return fmt.Sprintf("grp_%d_%x", time.Now().UTC().Unix(), buf), nil
}

// GeneratePublicSourceID creates a unique public source ID.
func GeneratePublicSourceID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating public source id: %w", err)
	}
	return fmt.Sprintf("pubsrc_%d_%x", time.Now().UTC().Unix(), buf), nil
}

// GenerateInboundID creates a unique inbound ID.
func GenerateInboundID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating inbound id: %w", err)
	}
	return fmt.Sprintf("inb_%d_%x", time.Now().UTC().Unix(), buf), nil
}

// NormalizeCountryCode uppercases a two-letter ISO 3166-1 alpha-2 code.
func NormalizeCountryCode(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 2 {
		return s
	}
	u := strings.ToUpper(s)
	if u[0] >= 'A' && u[0] <= 'Z' && u[1] >= 'A' && u[1] <= 'Z' {
		return u
	}
	return s
}
