package shared

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

// ID is the canonical identifier for every domain entity.
// It is a value type wrapping a UUID; the zero value is invalid
// and `IsZero` reports whether an ID has been initialised.
type ID struct {
	value uuid.UUID
}

// NewID generates a fresh random UUIDv4 ID.
func NewID() ID {
	return ID{value: uuid.New()}
}

// ParseID parses a textual representation of an ID. An empty string
// or any non-UUID input returns ErrInvalidID — callers should not
// expose the underlying parsing error to clients.
func ParseID(raw string) (ID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ID{}, ErrInvalidID
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return ID{}, ErrInvalidID
	}
	return ID{value: parsed}, nil
}

// MustParseID is the panic-on-error sibling of ParseID. Reserve for
// tests and constants where the input is statically known to parse.
func MustParseID(raw string) ID {
	id, err := ParseID(raw)
	if err != nil {
		panic(err)
	}
	return id
}

// String renders the ID in canonical UUID form.
func (i ID) String() string { return i.value.String() }

// UUID exposes the underlying value for adapters that must speak in
// UUIDs (e.g. the Postgres driver). Domain code should not call this.
func (i ID) UUID() uuid.UUID { return i.value }

// IsZero reports whether the ID has not been initialised.
func (i ID) IsZero() bool { return i.value == uuid.Nil }

// Equals reports whether two IDs reference the same entity.
func (i ID) Equals(other ID) bool { return i.value == other.value }

// MarshalJSON renders the ID as a canonical UUID string. Without
// this, the unexported `value` field would serialise to `{}` and
// every API response carrying an ID would be broken — the JSON
// contract (and the generated TypeScript client) depend on this.
// The zero ID marshals to JSON null.
func (i ID) MarshalJSON() ([]byte, error) {
	if i.value == uuid.Nil {
		return []byte("null"), nil
	}
	return []byte(`"` + i.value.String() + `"`), nil
}

// UnmarshalJSON parses a UUID string (or null) back into an ID. It
// reuses ParseID so malformed input yields the same ErrInvalidID
// sentinel the rest of the codebase already maps.
func (i *ID) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "null" || s == "" {
		*i = ID{}
		return nil
	}
	parsed, err := ParseID(s)
	if err != nil {
		return err
	}
	*i = parsed
	return nil
}

// ErrInvalidID is returned by ParseID when the input is not a UUID.
// It is a domain-shared sentinel so any layer can recognise it.
var ErrInvalidID = errors.New("shared: invalid id")
