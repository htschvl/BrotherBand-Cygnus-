package user_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	domainmedia "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	domainuser "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fakes"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fixtures"
	usecaseuser "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/user"
)

// ─── GetProfile ──────────────────────────────────────────────────────

func TestGetProfile_WhenFound_ReturnsView(t *testing.T) {
	t.Parallel()
	repo := fakes.NewUserRepo()
	seed := fixtures.NewUser().WithUsername("viewer").Build(t)
	if _, err := repo.Save(context.Background(), seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	uc := usecaseuser.NewGetProfile(repo, fakes.NewImageStore())

	view, err := uc.Execute(context.Background(), seed.ID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Username != "viewer" {
		t.Fatalf("wrong view: %#v", view)
	}
}

func TestGetProfile_WhenMissing_ReturnsNotFoundAndLogsWarn(t *testing.T) {
	t.Parallel()
	uc := usecaseuser.NewGetProfile(fakes.NewUserRepo(), fakes.NewImageStore())
	ctx, cap := ctxWithCapture()

	_, err := uc.Execute(ctx, fixtures.NewUser().Build(t).ID())
	if !errors.Is(err, domainuser.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	rec, ok := cap.FindByMessage("get_profile: lookup failed")
	if !ok || rec.Level != slog.LevelWarn {
		t.Fatalf("a /me 404 (deleted-user JWT) must log at WARN, got %+v ok=%v", rec, ok)
	}
}

// ─── UpdateStatus ────────────────────────────────────────────────────

func TestUpdateStatus_WhenValid_PersistsAndLogsInfo(t *testing.T) {
	t.Parallel()
	repo := fakes.NewUserRepo()
	seed := fixtures.NewUser().Build(t)
	if _, err := repo.Save(context.Background(), seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	uc := usecaseuser.NewUpdateStatus(repo)
	ctx, cap := ctxWithCapture()

	err := uc.Execute(ctx, usecaseuser.UpdateStatusInput{UserID: seed.ID(), Status: "feeling calm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := repo.FindByID(context.Background(), seed.ID())
	if got.Status().String() != "feeling calm" {
		t.Fatalf("status not persisted: %q", got.Status().String())
	}
	if _, ok := cap.FindByMessage("status updated"); !ok {
		t.Fatal("expected an INFO 'status updated' line")
	}
}

func TestUpdateStatus_WhenInvalid_Rejected(t *testing.T) {
	t.Parallel()
	uc := usecaseuser.NewUpdateStatus(fakes.NewUserRepo())
	err := uc.Execute(context.Background(), usecaseuser.UpdateStatusInput{
		UserID: fixtures.NewUser().Build(t).ID(),
		Status: "",
	})
	if !errors.Is(err, domainuser.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestUpdateStatus_WhenRepoFails_WrapsAndLogsError(t *testing.T) {
	t.Parallel()
	uc := usecaseuser.NewUpdateStatus(fakes.FailingUserRepo{})
	ctx, cap := ctxWithCapture()
	err := uc.Execute(ctx, usecaseuser.UpdateStatusInput{
		UserID: fixtures.NewUser().Build(t).ID(),
		Status: "valid status",
	})
	if !errors.Is(err, fakes.ErrInjected) {
		t.Fatalf("expected wrapped injected error, got %v", err)
	}
	if _, ok := cap.FindByMessage("update_status: repository failed"); !ok {
		t.Fatal("repo failure must log at ERROR")
	}
}

// ─── UpdateAvatar ────────────────────────────────────────────────────

func TestUpdateAvatar_WhenPendingKeyOwned_PromotesAndPersists(t *testing.T) {
	t.Parallel()
	repo := fakes.NewUserRepo()
	seed := fixtures.NewUser().Build(t)
	if _, err := repo.Save(context.Background(), seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	store := fakes.NewImageStore()
	uc := usecaseuser.NewUpdateAvatar(repo, store)
	ctx, cap := ctxWithCapture()

	pendingKey := "pending/" + seed.ID().String() + "/upload-1.webp"
	if err := uc.Execute(ctx, usecaseuser.UpdateAvatarInput{UserID: seed.ID(), MediaKey: pendingKey}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	final, ok := store.Promoted[pendingKey]
	if !ok {
		t.Fatal("the pending object must have been promoted")
	}
	got, _ := repo.FindByID(context.Background(), seed.ID())
	if got.AvatarKey() == nil || *got.AvatarKey() != final {
		t.Fatalf("avatar pointer not persisted: %v", got.AvatarKey())
	}
	if _, ok := cap.FindByMessage("avatar updated"); !ok {
		t.Fatal("expected an INFO 'avatar updated' line")
	}
}

func TestUpdateAvatar_WhenKeyNotOwned_RejectedAndLogsWarn(t *testing.T) {
	t.Parallel()
	store := fakes.NewImageStore()
	uc := usecaseuser.NewUpdateAvatar(fakes.NewUserRepo(), store)
	ctx, cap := ctxWithCapture()

	me := fixtures.NewUser().Build(t)
	someoneElse := fixtures.NewUser().WithUsername("other").Build(t)
	// Key belongs to a different user's pending prefix → broken access control.
	foreignKey := "pending/" + someoneElse.ID().String() + "/upload-9.webp"

	err := uc.Execute(ctx, usecaseuser.UpdateAvatarInput{UserID: me.ID(), MediaKey: foreignKey})
	if !errors.Is(err, domainmedia.ErrPromotionFailed) {
		t.Fatalf("expected ErrPromotionFailed, got %v", err)
	}
	if len(store.Promoted) != 0 {
		t.Fatal("a non-owned key must NOT be promoted")
	}
	rec, ok := cap.FindByMessage("update_avatar rejected: media key not owned by caller")
	if !ok || rec.Level != slog.LevelWarn {
		t.Fatalf("ownership rejection must log at WARN, got %+v ok=%v", rec, ok)
	}
}
