package shared_test

import (
	"encoding/json"
	"testing"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// The whole wire contract — and the generated TypeScript client —
// depends on IDs serialising as UUID strings, not as `{}`. This test
// guards the regression that the HTTP integration suite caught.

func TestID_MarshalsAsUUIDString(t *testing.T) {
	t.Parallel()
	id := shared.NewID()
	type payload struct {
		ID shared.ID `json:"id"`
	}
	raw, err := json.Marshal(payload{ID: id})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"id":"` + id.String() + `"}`
	if string(raw) != want {
		t.Fatalf("got %s, want %s", raw, want)
	}
}

func TestID_ZeroMarshalsAsNull(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(struct {
		ID shared.ID `json:"id"`
	}{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"id":null}` {
		t.Fatalf("zero ID must marshal to null, got %s", raw)
	}
}

func TestID_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := shared.NewID()
	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back shared.ID
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !back.Equals(orig) {
		t.Fatalf("round-trip mismatch: %v != %v", back, orig)
	}
}

func TestID_UnmarshalRejectsGarbage(t *testing.T) {
	t.Parallel()
	var id shared.ID
	if err := json.Unmarshal([]byte(`"not-a-uuid"`), &id); err == nil {
		t.Fatal("garbage UUID string must fail to unmarshal")
	}
}

func TestID_UnmarshalNullYieldsZero(t *testing.T) {
	t.Parallel()
	id := shared.NewID()
	if err := json.Unmarshal([]byte(`null`), &id); err != nil {
		t.Fatalf("null must unmarshal cleanly: %v", err)
	}
	if !id.IsZero() {
		t.Fatal("null must produce the zero ID")
	}
}
