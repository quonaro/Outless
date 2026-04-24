package domain

import "errors"

// ErrNodeNotFound is returned when a node does not exist in storage.
var ErrNodeNotFound = errors.New("node not found")

// ErrUnauthorized is returned when token validation fails.
var ErrUnauthorized = errors.New("unauthorized")

// ErrAdminAlreadyExists is returned when trying to register first admin after bootstrap is complete.
var ErrAdminAlreadyExists = errors.New("admin already exists")

// ErrAdminNotFound is returned when an admin is not found in storage.
var ErrAdminNotFound = errors.New("admin not found")

// ErrGroupNotFound is returned when a group is not found in storage.
var ErrGroupNotFound = errors.New("group not found")

// ErrTokenNotFound is returned when a token is not found in storage.
var ErrTokenNotFound = errors.New("token not found")

// ErrPublicSourceNotFound is returned when a public source is not found in storage.
var ErrPublicSourceNotFound = errors.New("public source not found")

// ErrDuplicateNode is returned when attempting to create a node that already exists.
var ErrDuplicateNode = errors.New("duplicate node")

// ErrProbeJobNotFound is returned when a probe job is not found in storage.
var ErrProbeJobNotFound = errors.New("probe job not found")
