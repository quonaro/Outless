package domain

import (
	"crypto/rand"
	"fmt"
	"time"
)

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

// GenerateGroupID creates a unique group ID.
func GenerateGroupID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating group id: %w", err)
	}
	return fmt.Sprintf("grp_%d_%x", time.Now().UTC().Unix(), buf), nil
}
