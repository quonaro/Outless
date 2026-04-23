package domain

import "time"

// Token describes an access token metadata.
// UUID is the per-token identifier used as VLESS user id on the Hub.
type Token struct {
	ID        string
	Owner     string
	GroupID   string
	UUID      string
	IsActive  bool
	ExpiresAt time.Time
	CreatedAt time.Time
}
