package brotherband_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domainbb "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/clock"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fakes"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fixtures"
	usecasebb "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/brotherband"
)

// TestAcceptRequest_RevealsRequesterSecretExactlyOnce is the
// load-bearing product invariant of the brotherband flow: when the
// recipient accepts, the requester's secret crosses the wire exactly
// once. After acceptance the request is gone — there is no path that
// can return the secret again.
func TestAcceptRequest_RevealsRequesterSecretExactlyOnce(t *testing.T) {
	t.Parallel()

	users := fakes.NewUserRepo()
	requests := fakes.NewRequestRepo()
	bonds := fakes.NewBrotherhoodRepo()
	images := fakes.NewImageStore()
	at := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	clk := clock.Fixed{At: at}

	requester := fixtures.NewUser().
		WithUsername("requester").
		WithSecret("the cat lives in the kettle").
		Build(t)
	recipient := fixtures.NewUser().WithUsername("recipient").Build(t)

	if _, err := users.Save(context.Background(), requester); err != nil {
		t.Fatalf("save requester: %v", err)
	}
	if _, err := users.Save(context.Background(), recipient); err != nil {
		t.Fatalf("save recipient: %v", err)
	}

	req, err := domainbb.NewRequest(requester.ID(), recipient.ID(), at)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	saved, err := requests.Save(context.Background(), req)
	if err != nil {
		t.Fatalf("save request: %v", err)
	}

	uc := usecasebb.NewAcceptRequest(requests, bonds, users, images, clk)
	out, err := uc.Execute(context.Background(), usecasebb.AcceptInput{
		UserID:    recipient.ID(),
		RequestID: saved.ID(),
	})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if out.RequesterSecret != requester.Secret().String() {
		t.Fatalf("expected secret %q, got %q", requester.Secret().String(), out.RequesterSecret)
	}
	if !out.Brother.ID.Equals(requester.ID()) {
		t.Fatalf("brother id mismatch")
	}

	// The request must be gone (one-shot reveal).
	if _, err := requests.FindByID(context.Background(), saved.ID()); !errors.Is(err, domainbb.ErrRequestNotFound) {
		t.Fatalf("expected request to be deleted, got %v", err)
	}
	// And the brotherhood must exist.
	exists, err := bonds.Exists(context.Background(), requester.ID(), recipient.ID())
	if err != nil || !exists {
		t.Fatalf("brotherhood should exist after accept")
	}
}

func TestAcceptRequest_OnlyRecipientCanAccept(t *testing.T) {
	t.Parallel()

	users := fakes.NewUserRepo()
	requests := fakes.NewRequestRepo()
	bonds := fakes.NewBrotherhoodRepo()
	images := fakes.NewImageStore()
	at := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	clk := clock.Fixed{At: at}

	requester := fixtures.NewUser().WithUsername("requester2").Build(t)
	recipient := fixtures.NewUser().WithUsername("recipient2").Build(t)
	bystander := fixtures.NewUser().WithUsername("bystander2").Build(t)

	if _, err := users.Save(context.Background(), requester); err != nil {
		t.Fatalf("save requester: %v", err)
	}
	if _, err := users.Save(context.Background(), recipient); err != nil {
		t.Fatalf("save recipient: %v", err)
	}
	if _, err := users.Save(context.Background(), bystander); err != nil {
		t.Fatalf("save bystander: %v", err)
	}

	req, _ := domainbb.NewRequest(requester.ID(), recipient.ID(), at)
	saved, _ := requests.Save(context.Background(), req)

	uc := usecasebb.NewAcceptRequest(requests, bonds, users, images, clk)
	_, err := uc.Execute(context.Background(), usecasebb.AcceptInput{
		UserID:    bystander.ID(),
		RequestID: saved.ID(),
	})
	if !errors.Is(err, domainbb.ErrNotRecipient) {
		t.Fatalf("expected ErrNotRecipient for non-recipient accepter, got %v", err)
	}
}
