package domain

import "errors"

// ErrNodeNotFound is returned when a node does not exist in storage.
var ErrNodeNotFound = errors.New("node not found")

// ErrUnauthorized is returned when token validation fails.
var ErrUnauthorized = errors.New("unauthorized")
