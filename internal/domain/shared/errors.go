package shared

import "errors"

// Base sentinels every aggregate can wrap with errors.Join or fmt.Errorf("%w", ...)
// so the HTTP error map can classify the result without knowing the originating
// aggregate. Aggregate-specific errors should wrap one of these so the upper
// layers can match by category as a fallback.
var (
	// ErrNotFound — the requested entity does not exist.
	ErrNotFound = errors.New("shared: not found")

	// ErrConflict — the requested change conflicts with current state.
	ErrConflict = errors.New("shared: conflict")

	// ErrForbidden — the actor is authenticated but not allowed.
	ErrForbidden = errors.New("shared: forbidden")

	// ErrInvalidInput — the input failed a domain invariant.
	ErrInvalidInput = errors.New("shared: invalid input")

	// ErrUnauthenticated — no valid actor was supplied.
	ErrUnauthenticated = errors.New("shared: unauthenticated")
)
