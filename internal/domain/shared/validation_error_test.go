package shared_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

func TestValidationError_IsInvalidInput(t *testing.T) {
	t.Parallel()
	err := shared.NewValidationError("username", "too short")
	if !errors.Is(err, shared.ErrInvalidInput) {
		t.Fatal("ValidationError must satisfy errors.Is(ErrInvalidInput)")
	}
	if err.Error() != "invalid username: too short" {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestValidationError_DefaultsFillBlankFields(t *testing.T) {
	t.Parallel()
	err := shared.NewValidationError("", "")
	if err.Field != "input" || err.Reason != "invalid value" {
		t.Fatalf("blank fields not defaulted: %#v", err)
	}
}

func TestWrapValidation_MatchesBothSentinelAndCategory(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("specific sentinel")
	err := shared.WrapValidation(sentinel, "password", "too weak")

	if !errors.Is(err, sentinel) {
		t.Fatal("must match the specific sentinel")
	}
	if !errors.Is(err, shared.ErrInvalidInput) {
		t.Fatal("must also match the broad category")
	}
}

func TestAsValidationError_UnwrapsThroughFmtErrorf(t *testing.T) {
	t.Parallel()
	base := shared.WrapValidation(shared.ErrInvalidInput, "body", "empty")
	wrapped := fmt.Errorf("handler: %w", base)

	ve, ok := shared.AsValidationError(wrapped)
	if !ok {
		t.Fatal("AsValidationError must see through fmt.Errorf wrapping")
	}
	if ve.Field != "body" || ve.Reason != "empty" {
		t.Fatalf("wrong fields extracted: %#v", ve)
	}
}

func TestAsValidationError_FalseForPlainError(t *testing.T) {
	t.Parallel()
	if _, ok := shared.AsValidationError(errors.New("nope")); ok {
		t.Fatal("plain errors are not ValidationErrors")
	}
}

func TestValidationErrors_AggregatesAndMatchesCategory(t *testing.T) {
	t.Parallel()
	if shared.NewValidationErrors(nil) != nil {
		t.Fatal("empty aggregate must be nil so the success path is nil")
	}
	agg := shared.NewValidationErrors([]*shared.ValidationError{
		shared.NewValidationError("a", "bad"),
		shared.NewValidationError("b", "worse"),
	})
	if !errors.Is(agg, shared.ErrInvalidInput) {
		t.Fatal("aggregate must satisfy the broad category")
	}
	if agg.Error() != "validation failed: invalid a: bad; invalid b: worse" {
		t.Fatalf("unexpected aggregate message: %q", agg.Error())
	}
}
