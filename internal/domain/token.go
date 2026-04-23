package domain

import "time"

// Token describes an access token metadata.
// UUID is the per-token identifier used as VLESS user id on the Hub.
type Token struct {
	ID    string
	Owner string
	// GroupID is kept for backward compatibility and mirrors the first item
	// from GroupIDs when any group is set.
	GroupID    string
	GroupIDs   []string
	TokenPlain string
	UUID       string
	IsActive   bool
	ExpiresAt  time.Time
	CreatedAt  time.Time
}
