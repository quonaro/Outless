package domain

import "time"

// Token describes an access token metadata.
type Token struct {
	ID        string
	Owner     string
	IsActive  bool
	ExpiresAt time.Time
	CreatedAt time.Time
}
