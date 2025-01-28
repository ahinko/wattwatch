package provider

import "errors"

var (
	// ErrProviderNotFound is returned when a provider cannot be found by name
	ErrProviderNotFound = errors.New("provider not found")
)
