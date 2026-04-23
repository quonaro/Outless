package domain

import "time"

// Group represents a collection of nodes and tokens for access control.
type Group struct {
	ID           string
	Name         string
	SourceURL    string
	LastSyncedAt *time.Time
	CreatedAt    time.Time
}
