package domain

import (
	"crypto/rand"
	"fmt"
	"time"
)

// PublicSource represents an external source of VLESS nodes.
type PublicSource struct {
	ID            string
	URL           string
	GroupID       string
	LastFetchedAt *time.Time
	CreatedAt     time.Time
}

// GeneratePublicSourceID creates a unique public source ID.
func GeneratePublicSourceID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating public source id: %w", err)
	}
	return fmt.Sprintf("pubsrc_%d_%x", time.Now().UTC().Unix(), buf), nil
}
