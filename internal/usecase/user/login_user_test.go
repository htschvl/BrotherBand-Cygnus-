package user_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	domainuser "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/clock"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fakes"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fixtures"
	usecaseuser "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/user"
)

func newLoginUC(repo *fakes.UserRepo) *usecaseuser.LoginUser {
	return usecaseuser.NewLoginUser(
		repo,
		fakes.StaticHasher{},
		fakes.NewTokenIssuer(),
		fakes.NewCSRFMinter("csrf"),
		clock.Fixed{At: time.Now()},
		fakes.NewImageStore(),
	)
}

func TestLoginUser_WhenValid_SucceedsAndLogsInfo(t *testing.T) {
	t.Parallel()
	repo := fakes.NewUserRepo()
	// fixtures store passwordHash = "hash:" + password, matching StaticHasher.
	seed := fixtures.NewUser().WithUsername("alice").WithPassword("Hunter2!Hunter2").Build(t)
	if _, err := repo.Save(context.Background(), seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	uc := newLoginUC(repo)
	ctx, cap := ctxWithCapture()

	session, err := uc.Execute(ctx, usecaseuser.LoginUserInput{Username: "alice", Password: "Hunter2!Hunter2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Token.Value == "" {
		t.Fatal("expected a session token")
	}
	rec, ok := cap.FindByMessage("user authenticated")
	if !ok || rec.Level != slog.LevelInfo || rec.Attrs["event"] != "auth.login.succeeded" {
		t.Fatalf("login success must log INFO with event attr, got %+v ok=%v", rec, ok)
	}
}

func TestLoginUser_FailureModesCollapseToInvalidCredentials(t *testing.T) {
	t.Parallel()
	repo := fakes.NewUserRepo()
	seed := fixtures.NewUser().WithUsername("bob").WithPassword("Hunter2!Hunter2").Build(t)
	if _, err := repo.Save(context.Background(), seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	uc := newLoginUC(repo)

	cases := []struct {
		name   string
		in     usecaseuser.LoginUserInput
		logMsg string
	}{
		{"unknown_user", usecaseuser.LoginUserInput{Username: "ghost", Password: "whatever123"}, "login failed: unknown username"},
		{"bad_password", usecaseuser.LoginUserInput{Username: "bob", Password: "wrongpassword"}, "login failed: bad password"},
		{"malformed_username", usecaseuser.LoginUserInput{Username: "x", Password: "y"}, "login rejected: malformed username"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cap := ctxWithCapture()
			_, err := uc.Execute(ctx, tc.in)
			if !errors.Is(err, domainuser.ErrInvalidCredentials) {
				t.Fatalf("every failure must collapse to ErrInvalidCredentials, got %v", err)
			}
			if _, ok := cap.FindByMessage(tc.logMsg); !ok {
				t.Fatalf("expected internal log %q distinguishing the cause", tc.logMsg)
			}
		})
	}
}

func TestLoginUser_WhenRepositoryErrors_WrapsAndLogsError(t *testing.T) {
	t.Parallel()
	uc := usecaseuser.NewLoginUser(
		fakes.FailingUserRepo{},
		fakes.StaticHasher{},
		fakes.NewTokenIssuer(),
		fakes.NewCSRFMinter("csrf"),
		clock.Fixed{At: time.Now()},
		fakes.NewImageStore(),
	)
	ctx, cap := ctxWithCapture()
	_, err := uc.Execute(ctx, usecaseuser.LoginUserInput{Username: "alice", Password: "Hunter2!Hunter2"})
	if !errors.Is(err, fakes.ErrInjected) {
		t.Fatalf("a repo failure must be wrapped, got %v", err)
	}
	if _, ok := cap.FindByMessage("login: repository error"); !ok {
		t.Fatal("unexpected repo error must log at ERROR")
	}
}
