package domain

import "errors"

// Sentinel errors used across the application.
var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrInvalidInput    = errors.New("invalid input")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrCrossTenant     = errors.New("cross-tenant access denied")
	ErrNoConfigVersion = errors.New("no active config version for tenant")
)
