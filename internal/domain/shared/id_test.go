package shared_test

import (
	"errors"
	"testing"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

func TestNewID_IsNotZero(t *testing.T) {
	t.Parallel()
	if shared.NewID().IsZero() {
		t.Fatal("expected fresh ID to be non-zero")
	}
}

func TestParseID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid_uuid_v4", "0192e2c1-1c2c-7e9d-aaaa-000000000001", false},
		{"empty", "", true},
		{"whitespace", "   ", true},
		{"garbage", "not-a-uuid", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := shared.ParseID(tc.input)
			if tc.wantErr && !errors.Is(err, shared.ErrInvalidID) {
				t.Fatalf("expected ErrInvalidID, got %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestID_EqualsAndRoundtrip(t *testing.T) {
	t.Parallel()
	id := shared.NewID()
	parsed, err := shared.ParseID(id.String())
	if err != nil {
		t.Fatalf("parse own string: %v", err)
	}
	if !id.Equals(parsed) {
		t.Fatalf("round-trip mismatch")
	}
	if id.Equals(shared.NewID()) {
		t.Fatal("two fresh IDs should not be equal")
	}
}
