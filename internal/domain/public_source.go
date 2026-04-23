package domain

import "time"

// PublicSource represents an external source of VLESS nodes.
type PublicSource struct {
	ID            string
	URL           string
	GroupID       string
	LastFetchedAt *time.Time
	CreatedAt     time.Time
}
