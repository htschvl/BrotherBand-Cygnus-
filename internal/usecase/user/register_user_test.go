package user_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	domainuser "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/clock"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fakes"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fixtures"
	usecaseuser "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/user"
)

func validRegisterInput() usecaseuser.RegisterUserInput {
	return usecaseuser.RegisterUserInput{
		Username:  "new_user",
		Password:  "Hunter2!Hunter2",
		Birthdate: "1994-03-21",
		Secret:    "the owl knows the way home",
		Status:    "new here",
		Favorites: []string{"tea", "rain", "vinyl", "bread", "moss"},
	}
}

func newRegisterUC(repo *fakes.UserRepo) (*usecaseuser.RegisterUser, *fakes.ImageStore) {
	img := fakes.NewImageStore()
	uc := usecaseuser.NewRegisterUser(
		repo, repo,
		fakes.StaticHasher{},
		fakes.NewTokenIssuer(),
		fakes.NewCSRFMinter("csrf"),
		clock.Fixed{At: time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)},
		img,
	)
	return uc, img
}

func ctxWithCapture() (context.Context, *logging.Capture) {
	cap := logging.NewCapture(slog.LevelDebug)
	return logging.WithLogger(context.Background(), cap.Logger()), cap
}

func TestRegisterUser_WhenValid_SucceedsAndLogsInfo(t *testing.T) {
	t.Parallel()
	repo := fakes.NewUserRepo()
	uc, _ := newRegisterUC(repo)
	ctx, cap := ctxWithCapture()

	session, err := uc.Execute(ctx, validRegisterInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Profile.Username != "new_user" {
		t.Fatalf("wrong profile: %#v", session.Profile)
	}
	if session.Token.Value == "" || session.CSRFToken == "" {
		t.Fatal("session must carry token + csrf")
	}
	rec, ok := cap.FindByMessage("user registered")
	if !ok {
		t.Fatal("expected an INFO 'user registered' log line")
	}
	if rec.Level != slog.LevelInfo {
		t.Fatalf("registration success must log at INFO, got %v", rec.Level)
	}
	if rec.Attrs["event"] != "user.registered" {
		t.Fatalf("missing event attr: %#v", rec.Attrs)
	}
}

func TestRegisterUser_WhenUsernameTaken_ReturnsConflictAndLogsWarn(t *testing.T) {
	t.Parallel()
	repo := fakes.NewUserRepo()
	seed := fixtures.NewUser().WithUsername("taken").Build(t)
	if _, err := repo.Save(context.Background(), seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	uc, _ := newRegisterUC(repo)
	ctx, cap := ctxWithCapture()

	in := validRegisterInput()
	in.Username = "taken"
	_, err := uc.Execute(ctx, in)
	if !errors.Is(err, domainuser.ErrUsernameAlreadyTaken) {
		t.Fatalf("expected ErrUsernameAlreadyTaken, got %v", err)
	}
	rec, ok := cap.FindByMessage("register rejected: username taken")
	if !ok || rec.Level != slog.LevelWarn {
		t.Fatalf("username conflict must log at WARN, got %+v ok=%v", rec, ok)
	}
}

func TestRegisterUser_ValidationRejections(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		mutar func(*usecaseuser.RegisterUserInput)
		want  error
	}{
		{"weak_password", func(i *usecaseuser.RegisterUserInput) { i.Password = "short" }, domainuser.ErrPasswordTooWeak},
		{"bad_username", func(i *usecaseuser.RegisterUserInput) { i.Username = "x" }, domainuser.ErrInvalidUsername},
		{"bad_birthdate", func(i *usecaseuser.RegisterUserInput) { i.Birthdate = "nope" }, domainuser.ErrInvalidBirthdate},
		{"bad_secret", func(i *usecaseuser.RegisterUserInput) { i.Secret = "" }, domainuser.ErrInvalidSecret},
		{"bad_status", func(i *usecaseuser.RegisterUserInput) { i.Status = "" }, domainuser.ErrInvalidStatus},
		{"bad_favorites", func(i *usecaseuser.RegisterUserInput) { i.Favorites = []string{"only", "four", "items", "here"} }, domainuser.ErrInvalidFavorites},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := fakes.NewUserRepo()
			uc, _ := newRegisterUC(repo)
			in := validRegisterInput()
			tc.mutar(&in)
			_, err := uc.Execute(context.Background(), in)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRegisterUser_WhenHasherFails_ReturnsWrappedErrorAndLogsError(t *testing.T) {
	t.Parallel()
	repo := fakes.NewUserRepo()
	uc := usecaseuser.NewRegisterUser(
		repo, repo,
		fakes.FailingHasher{},
		fakes.NewTokenIssuer(),
		fakes.NewCSRFMinter("csrf"),
		clock.Fixed{At: time.Now()},
		fakes.NewImageStore(),
	)
	ctx, cap := ctxWithCapture()

	_, err := uc.Execute(ctx, validRegisterInput())
	if err == nil {
		t.Fatal("expected an error when the hasher fails")
	}
	if !errors.Is(err, fakes.ErrInjected) {
		t.Fatalf("error must wrap the injected cause, got %v", err)
	}
	if _, ok := cap.FindByMessage("register: password hashing failed"); !ok {
		t.Fatal("hasher failure must log at ERROR with a clear message")
	}
}

func TestRegisterUser_WhenContextCancelled_FailsFast(t *testing.T) {
	t.Parallel()
	repo := fakes.NewUserRepo()
	uc, _ := newRegisterUC(repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := uc.Execute(ctx, validRegisterInput())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
