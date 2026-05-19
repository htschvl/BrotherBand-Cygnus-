package message_test

import (
	"errors"
	"testing"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

func TestCursor_Roundtrip(t *testing.T) {
	t.Parallel()
	c := message.Cursor{
		CreatedAt: time.Date(2026, 5, 11, 12, 30, 45, 0, time.UTC),
		ID:        shared.NewID(),
	}
	encoded := c.Encode()
	if encoded == "" {
		t.Fatal("encoded cursor should not be empty")
	}
	decoded, err := message.DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded == nil {
		t.Fatal("decoded cursor should not be nil")
	}
	if !decoded.CreatedAt.Equal(c.CreatedAt) {
		t.Errorf("createdAt mismatch: %v vs %v", decoded.CreatedAt, c.CreatedAt)
	}
	if !decoded.ID.Equals(c.ID) {
		t.Errorf("id mismatch: %v vs %v", decoded.ID, c.ID)
	}
}

func TestDecodeCursor_EmptyMeansFirstPage(t *testing.T) {
	t.Parallel()
	got, err := message.DecodeCursor("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for empty cursor, got %+v", got)
	}
}

func TestDecodeCursor_RejectsGarbage(t *testing.T) {
	t.Parallel()
	if _, err := message.DecodeCursor("not-base64!"); !errors.Is(err, message.ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestNewBody(t *testing.T) {
	t.Parallel()
	if _, err := message.NewBody(""); !errors.Is(err, message.ErrInvalidBody) {
		t.Fatalf("empty body should be rejected, got %v", err)
	}
	if _, err := message.NewBody("   "); !errors.Is(err, message.ErrInvalidBody) {
		t.Fatalf("whitespace-only body should be rejected, got %v", err)
	}
	if _, err := message.NewBody("hello"); err != nil {
		t.Fatalf("valid body rejected: %v", err)
	}
}
