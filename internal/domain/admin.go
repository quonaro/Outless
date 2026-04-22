package domain

import "time"

// Admin represents an administrative user with access to management endpoints.
type Admin struct {
	ID        string
	Username  string
	PasswordHash string
	CreatedAt time.Time
}
