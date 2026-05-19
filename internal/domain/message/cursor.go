package message

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// Cursor is the opaque pagination token exchanged with clients.
// It encodes the (created_at, id) pair of the last seen message so
// the next page can use the keyset predicate from message.sql.
//
// The opacity is enforced by base64-encoding a JSON payload — the
// client must not parse it. If the schema ever changes, a `v` field
// can be added without breaking older clients (read code can branch
// on absent → v1).
type Cursor struct {
	CreatedAt time.Time `json:"c"`
	ID        shared.ID `json:"i"`
}

type cursorWire struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// Encode returns the base64 JSON form for transport.
func (c Cursor) Encode() string {
	wire := cursorWire{CreatedAt: c.CreatedAt, ID: c.ID.String()}
	raw, _ := json.Marshal(wire) // marshal of two known fields cannot fail
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeCursor parses the wire form back into a typed Cursor. An
// empty input produces a nil cursor (i.e. "first page").
func DecodeCursor(encoded string) (*Cursor, error) {
	if encoded == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var wire cursorWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, ErrInvalidCursor
	}
	id, err := shared.ParseID(wire.ID)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	return &Cursor{CreatedAt: wire.CreatedAt, ID: id}, nil
}
