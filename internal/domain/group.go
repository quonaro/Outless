package domain

import "time"

// Group represents a collection of nodes and tokens for access control.
type Group struct {
	ID                    string
	Name                  string
	SourceURL             string
	TotalNodes            int
	AutoDeleteUnavailable bool
	LastSyncedAt          *time.Time
	CreatedAt             time.Time
}
