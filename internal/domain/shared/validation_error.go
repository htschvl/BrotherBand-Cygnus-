package shared

import (
	"errors"
	"fmt"
	"strings"
)

// ValidationError is the typed error every value-object constructor
// returns when its input fails an invariant. It carries the field
// name, a human-readable reason, and optionally a more specific
// sentinel (e.g. `user.ErrPasswordTooWeak`) so the HTTP layer can
// render 422 responses with field-level detail AND any layer can
// match on the precise sentinel via errors.Is.
//
// A ValidationError ALWAYS satisfies `errors.Is(err, ErrInvalidInput)`,
// and when Sentinel is set it also satisfies `errors.Is(err, Sentinel)`.
type ValidationError struct {
	Field    string
	Reason   string
	Sentinel error
}

// NewValidationError is the canonical constructor. Both Field and
// Reason are required so a forgetful caller cannot produce an
// unhelpful error.
func NewValidationError(field, reason string) *ValidationError {
	if field == "" {
		field = "input"
	}
	if reason == "" {
		reason = "invalid value"
	}
	return &ValidationError{Field: field, Reason: reason}
}

// WrapValidation produces a ValidationError that also carries a more
// specific sentinel — `errors.Is(err, sentinel)` succeeds in addition
// to the broad ErrInvalidInput category.
func WrapValidation(sentinel error, field, reason string) *ValidationError {
	v := NewValidationError(field, reason)
	v.Sentinel = sentinel
	return v
}

// Error renders the canonical message used by error logs and the
// fallback "message" field in the HTTP error body.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

// Unwrap returns both the broad category and (when present) the
// specific sentinel so errors.Is matches either.
func (e *ValidationError) Unwrap() []error {
	if e.Sentinel != nil {
		return []error{ErrInvalidInput, e.Sentinel}
	}
	return []error{ErrInvalidInput}
}

// AsValidationError extracts a ValidationError from a wrapped error
// chain. The HTTP error map uses this to populate the `details.field`
// response attribute.
func AsValidationError(err error) (*ValidationError, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}

// ValidationErrors aggregates multiple field-level failures so a
// single validation pass can surface every problem at once instead
// of forcing the client to fix-and-retry one at a time.
type ValidationErrors struct{ Items []*ValidationError }

// NewValidationErrors returns nil if `items` is empty so callers can
// idiomatically write `return shared.NewValidationErrors(errs)` and
// have the success path be `nil`.
func NewValidationErrors(items []*ValidationError) error {
	if len(items) == 0 {
		return nil
	}
	return &ValidationErrors{Items: items}
}

func (e *ValidationErrors) Error() string {
	parts := make([]string, 0, len(e.Items))
	for _, it := range e.Items {
		parts = append(parts, it.Error())
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Unwrap satisfies errors.Is for the shared category.
func (e *ValidationErrors) Unwrap() error { return ErrInvalidInput }
