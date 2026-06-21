package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewAdminID generates a unique admin identifier.
func NewAdminID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("admin_%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// NewID generates a random hex identifier of 16 bytes.
func NewID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("id_%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
