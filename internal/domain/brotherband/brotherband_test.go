package brotherband_test

import (
	"errors"
	"testing"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

func TestNewRequest_RejectsSelfRequest(t *testing.T) {
	t.Parallel()
	id := shared.NewID()
	_, err := brotherband.NewRequest(id, id, time.Now())
	if !errors.Is(err, brotherband.ErrSelfRequest) {
		t.Fatalf("expected ErrSelfRequest, got %v", err)
	}
}

func TestBrotherhood_OrderIndependentEqualityViaIncludes(t *testing.T) {
	t.Parallel()
	a, b := shared.NewID(), shared.NewID()
	bond, err := brotherband.NewBrotherhood(a, b, time.Now())
	if err != nil {
		t.Fatalf("new brotherhood: %v", err)
	}
	if !bond.Includes(a) || !bond.Includes(b) {
		t.Fatalf("brotherhood should include both members")
	}
	if !bond.Other(a).Equals(b) {
		t.Fatalf("other(a) should equal b")
	}
	if !bond.Other(b).Equals(a) {
		t.Fatalf("other(b) should equal a")
	}
}
