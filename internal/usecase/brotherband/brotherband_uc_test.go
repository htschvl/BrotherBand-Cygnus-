package brotherband_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	domainbb "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	domainuser "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/clock"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fakes"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fixtures"
	usecasebb "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/brotherband"
)

func capCtx(t *testing.T) (context.Context, *logging.Capture) {
	t.Helper()
	c := logging.NewCapture(slog.LevelDebug)
	return logging.WithLogger(context.Background(), c.Logger()), c
}

var fixedClock = clock.Fixed{At: time.Date(2026, 5, 16, 9, 0, 0, 0, time.UTC)}

func seedTwoUsers(t *testing.T) (*fakes.UserRepo, domainuser.User, domainuser.User) {
	t.Helper()
	repo := fakes.NewUserRepo()
	a := fixtures.NewUser().WithUsername("alice").Build(t)
	b := fixtures.NewUser().WithUsername("bob").Build(t)
	if _, err := repo.Save(context.Background(), a); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if _, err := repo.Save(context.Background(), b); err != nil {
		t.Fatalf("seed b: %v", err)
	}
	return repo, a, b
}

// ─── SendRequest ─────────────────────────────────────────────────────

func TestSendRequest_HappyPath_LogsInfo(t *testing.T) {
	t.Parallel()
	users, a, b := seedTwoUsers(t)
	reqs := fakes.NewRequestRepo()
	bonds := fakes.NewBrotherhoodRepo()
	uc := usecasebb.NewSendRequest(reqs, bonds, users, fixedClock)
	ctx, c := capCtx(t)

	out, err := uc.Execute(ctx, usecasebb.SendRequestInput{RequesterID: a.ID(), RecipientID: b.ID()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.RequesterUsername != "alice" || out.RecipientUsername != "bob" {
		t.Fatalf("usernames not resolved: %#v", out)
	}
	if _, ok := c.FindByMessage("brotherband request sent"); !ok {
		t.Fatal("expected INFO 'brotherband request sent'")
	}
}

func TestSendRequest_SelfRequest_Rejected(t *testing.T) {
	t.Parallel()
	users, a, _ := seedTwoUsers(t)
	uc := usecasebb.NewSendRequest(fakes.NewRequestRepo(), fakes.NewBrotherhoodRepo(), users, fixedClock)
	_, err := uc.Execute(context.Background(), usecasebb.SendRequestInput{RequesterID: a.ID(), RecipientID: a.ID()})
	if !errors.Is(err, domainbb.ErrSelfRequest) {
		t.Fatalf("expected ErrSelfRequest, got %v", err)
	}
}

func TestSendRequest_RecipientMissing_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	users, a, _ := seedTwoUsers(t)
	uc := usecasebb.NewSendRequest(fakes.NewRequestRepo(), fakes.NewBrotherhoodRepo(), users, fixedClock)
	ghost := fixtures.NewUser().WithUsername("ghost").Build(t)
	_, err := uc.Execute(context.Background(), usecasebb.SendRequestInput{RequesterID: a.ID(), RecipientID: ghost.ID()})
	if !errors.Is(err, domainuser.ErrNotFound) {
		t.Fatalf("expected user.ErrNotFound, got %v", err)
	}
}

func TestSendRequest_AlreadyBrothers_Conflict(t *testing.T) {
	t.Parallel()
	users, a, b := seedTwoUsers(t)
	bonds := fakes.NewBrotherhoodRepo()
	bond, _ := domainbb.NewBrotherhood(a.ID(), b.ID(), fixedClock.Now())
	_ = bonds.Save(context.Background(), bond)
	uc := usecasebb.NewSendRequest(fakes.NewRequestRepo(), bonds, users, fixedClock)
	_, err := uc.Execute(context.Background(), usecasebb.SendRequestInput{RequesterID: a.ID(), RecipientID: b.ID()})
	if !errors.Is(err, domainbb.ErrAlreadyBrothers) {
		t.Fatalf("expected ErrAlreadyBrothers, got %v", err)
	}
}

func TestSendRequest_DuplicateRequest_Conflict(t *testing.T) {
	t.Parallel()
	users, a, b := seedTwoUsers(t)
	reqs := fakes.NewRequestRepo()
	uc := usecasebb.NewSendRequest(reqs, fakes.NewBrotherhoodRepo(), users, fixedClock)
	in := usecasebb.SendRequestInput{RequesterID: a.ID(), RecipientID: b.ID()}
	if _, err := uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("first send: %v", err)
	}
	_, err := uc.Execute(context.Background(), in)
	if !errors.Is(err, domainbb.ErrRequestExists) {
		t.Fatalf("expected ErrRequestExists, got %v", err)
	}
}

func TestSendRequest_RepoFailure_WrappedAndLoggedError(t *testing.T) {
	t.Parallel()
	users, a, b := seedTwoUsers(t)
	uc := usecasebb.NewSendRequest(fakes.NewRequestRepo(), fakes.FailingBrotherhoodRepo{}, users, fixedClock)
	ctx, c := capCtx(t)
	_, err := uc.Execute(ctx, usecasebb.SendRequestInput{RequesterID: a.ID(), RecipientID: b.ID()})
	if !errors.Is(err, fakes.ErrInjected) {
		t.Fatalf("expected wrapped injected error, got %v", err)
	}
	if _, ok := c.FindByMessage("send_request: brotherhood probe failed"); !ok {
		t.Fatal("probe failure must log at ERROR")
	}
}

// ─── DenyRequest ─────────────────────────────────────────────────────

func TestDenyRequest_OnlyRecipientCanDeny(t *testing.T) {
	t.Parallel()
	reqs := fakes.NewRequestRepo()
	a, b := fixtures.NewUser().Build(t), fixtures.NewUser().WithUsername("bob").Build(t)
	req, _ := domainbb.NewRequest(a.ID(), b.ID(), fixedClock.Now())
	saved, _ := reqs.Save(context.Background(), req)
	uc := usecasebb.NewDenyRequest(reqs)

	// requester tries to deny their own outbound request → forbidden
	if err := uc.Execute(context.Background(), usecasebb.DenyInput{UserID: a.ID(), RequestID: saved.ID()}); !errors.Is(err, domainbb.ErrNotRecipient) {
		t.Fatalf("expected ErrNotRecipient, got %v", err)
	}
	// recipient denies → ok
	if err := uc.Execute(context.Background(), usecasebb.DenyInput{UserID: b.ID(), RequestID: saved.ID()}); err != nil {
		t.Fatalf("recipient deny should succeed: %v", err)
	}
	if _, err := reqs.FindByID(context.Background(), saved.ID()); !errors.Is(err, domainbb.ErrRequestNotFound) {
		t.Fatal("request should be gone after deny")
	}
}

func TestDenyRequest_MissingRequest_NotFound(t *testing.T) {
	t.Parallel()
	uc := usecasebb.NewDenyRequest(fakes.NewRequestRepo())
	err := uc.Execute(context.Background(), usecasebb.DenyInput{
		UserID:    fixtures.NewUser().Build(t).ID(),
		RequestID: fixtures.NewUser().Build(t).ID(),
	})
	if !errors.Is(err, domainbb.ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got %v", err)
	}
}

// ─── CutBrotherband ──────────────────────────────────────────────────

func TestCutBrotherband_WhenBrothers_DeletesAndLogsInfo(t *testing.T) {
	t.Parallel()
	bonds := fakes.NewBrotherhoodRepo()
	a, b := fixtures.NewUser().Build(t), fixtures.NewUser().WithUsername("bob").Build(t)
	bond, _ := domainbb.NewBrotherhood(a.ID(), b.ID(), fixedClock.Now())
	_ = bonds.Save(context.Background(), bond)
	uc := usecasebb.NewCutBrotherband(bonds)
	ctx, c := capCtx(t)

	if err := uc.Execute(ctx, usecasebb.CutInput{UserID: b.ID(), BrotherID: a.ID()}); err != nil {
		t.Fatalf("cut should succeed (order-independent): %v", err)
	}
	if ok, _ := bonds.Exists(context.Background(), a.ID(), b.ID()); ok {
		t.Fatal("brotherhood should be gone")
	}
	if _, ok := c.FindByMessage("brotherhood cut"); !ok {
		t.Fatal("expected INFO 'brotherhood cut'")
	}
}

func TestCutBrotherband_WhenNotBrothers_Forbidden(t *testing.T) {
	t.Parallel()
	uc := usecasebb.NewCutBrotherband(fakes.NewBrotherhoodRepo())
	err := uc.Execute(context.Background(), usecasebb.CutInput{
		UserID:    fixtures.NewUser().Build(t).ID(),
		BrotherID: fixtures.NewUser().Build(t).ID(),
	})
	if !errors.Is(err, domainbb.ErrNotABrother) {
		t.Fatalf("expected ErrNotABrother, got %v", err)
	}
}

// ─── ListRequests / ListBrothers / GetBrother ────────────────────────

func TestListRequests_SplitsByDirection(t *testing.T) {
	t.Parallel()
	_, a, b := seedTwoUsers(t)
	reqs := fakes.NewRequestRepo()
	r1, _ := domainbb.NewRequest(a.ID(), b.ID(), fixedClock.Now()) // a → b (sent by a)
	r2, _ := domainbb.NewRequest(b.ID(), a.ID(), fixedClock.Now()) // b → a (received by a)
	_, _ = reqs.Save(context.Background(), r1)
	_, _ = reqs.Save(context.Background(), r2)
	uc := usecasebb.NewListRequests(reqs)

	out, err := uc.Execute(context.Background(), usecasebb.ListRequestsInput{UserID: a.ID(), Direction: "all"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Sent) != 1 || len(out.Received) != 1 {
		t.Fatalf("expected 1 sent + 1 received, got %d/%d", len(out.Sent), len(out.Received))
	}

	onlySent, _ := uc.Execute(context.Background(), usecasebb.ListRequestsInput{UserID: a.ID(), Direction: "sent"})
	if len(onlySent.Sent) != 1 || onlySent.Received != nil {
		t.Fatalf("direction=sent must omit received: %#v", onlySent)
	}
}

func TestListBrothers_ReturnsConfirmed(t *testing.T) {
	t.Parallel()
	bonds := fakes.NewBrotherhoodRepo()
	a, b := fixtures.NewUser().Build(t), fixtures.NewUser().WithUsername("bob").Build(t)
	bonds.SetUserLookup(func(id shared.ID) domainbb.Brother {
		return domainbb.Brother{ID: id, Username: "bob", Status: "ok"}
	})
	bond, _ := domainbb.NewBrotherhood(a.ID(), b.ID(), fixedClock.Now())
	_ = bonds.Save(context.Background(), bond)
	uc := usecasebb.NewListBrothers(bonds, fakes.NewImageStore())

	out, err := uc.Execute(context.Background(), a.ID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected exactly one brother, got %d", len(out))
	}
}

func TestGetBrother_WhenNotBrothers_Forbidden(t *testing.T) {
	t.Parallel()
	uc := usecasebb.NewGetBrother(fakes.NewBrotherhoodRepo(), fakes.NewUserRepo(), fakes.NewImageStore())
	_, err := uc.Execute(context.Background(),
		fixtures.NewUser().Build(t).ID(),
		fixtures.NewUser().Build(t).ID(),
	)
	if !errors.Is(err, domainbb.ErrNotABrother) {
		t.Fatalf("expected ErrNotABrother, got %v", err)
	}
}
