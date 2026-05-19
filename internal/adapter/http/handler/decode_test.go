package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

type sample struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// decodeJSON is unexported, so this is a white-box test in package
// handler. It exercises every branch of classifyDecodeError, because
// the JSON decoder's native error messages are notoriously unhelpful
// and the translation is the whole point.
func TestDecodeJSON_Classification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		body       string
		wantField  string
		wantReason string // substring
	}{
		{"empty_body", "", "body", "empty"},
		{"malformed_json", `{"name":`, "body", "malformed JSON"},
		{"wrong_type", `{"name":"ok","age":"NaN"}`, "age", "expected int"},
		{"unknown_field", `{"name":"ok","nope":1}`, "nope", "not allowed"},
		{"trailing_data", `{"name":"ok"} garbage`, "body", "unexpected data"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			var dst sample
			err := decodeJSON(req, &dst)
			if err == nil {
				t.Fatalf("expected an error for %q", tc.body)
			}
			ve, ok := shared.AsValidationError(err)
			if !ok {
				t.Fatalf("decode error must be a ValidationError, got %T: %v", err, err)
			}
			if ve.Field != tc.wantField {
				t.Errorf("field: got %q want %q", ve.Field, tc.wantField)
			}
			if !strings.Contains(ve.Reason, tc.wantReason) {
				t.Errorf("reason: got %q want substring %q", ve.Reason, tc.wantReason)
			}
		})
	}
}

func TestDecodeJSON_HappyPath(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"ok","age":7}`))
	var dst sample
	if err := decodeJSON(req, &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "ok" || dst.Age != 7 {
		t.Fatalf("decoded wrong values: %#v", dst)
	}
}

func TestDecodeJSON_RejectsOversizedBody(t *testing.T) {
	t.Parallel()
	huge := `{"name":"` + strings.Repeat("x", int(MaxRequestBodyBytes)+10) + `"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(huge))
	var dst sample
	err := decodeJSON(req, &dst)
	if err == nil {
		t.Fatal("expected oversized body to be rejected")
	}
	ve, ok := shared.AsValidationError(err)
	if !ok || ve.Field != "body" {
		t.Fatalf("expected a body ValidationError, got %v", err)
	}
	if !strings.Contains(ve.Reason, "at most") {
		t.Fatalf("reason should mention the size limit, got %q", ve.Reason)
	}
}
