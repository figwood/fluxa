package shared

import "errors"

var (
	ErrNotFound          = errors.New("not_found")
	ErrInvalidTransition = errors.New("invalid_transition")
	ErrInvalidInput      = errors.New("invalid_input")
	ErrConflict          = errors.New("conflict")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
)
