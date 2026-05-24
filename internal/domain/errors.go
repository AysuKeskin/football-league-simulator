package domain

import "errors"

// Sentinel errors that services return to indicate well-known failure
// modes. Handlers map these to HTTP status codes at the edge, so
// neither services nor repositories need to know anything about HTTP.
//
// Wrap with fmt.Errorf("%w: ...", ErrXxx) to add context while keeping
// errors.Is recognition intact.
var (
	// ErrNotFound indicates a requested entity does not exist.
	ErrNotFound = errors.New("not found")

	// ErrConflict indicates the operation conflicts with current state
	// (e.g. playing a week on a finished league, editing a scheduled match).
	ErrConflict = errors.New("conflict")

	// ErrInvalidInput indicates the caller supplied invalid arguments
	// that validation could not catch at the transport layer.
	ErrInvalidInput = errors.New("invalid input")
)
