package errors

import "errors"

var (
	// ErrNotImplemented is returned when a feature or integration is in a stubbed state.
	ErrNotImplemented = errors.New("feature not implemented")

	// ErrNotFound is returned when a record is not found in the storage.
	ErrNotFound = errors.New("resource not found")

	// ErrAlreadyExists is returned when a unique constraint is violated.
	ErrAlreadyExists = errors.New("resource already exists")
)
