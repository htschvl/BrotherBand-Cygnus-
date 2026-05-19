package user

import (
	"context"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// Reader is the read-only slice of the user repository, consumed by
// authentication and profile-lookup use cases.
type Reader interface {
	FindByID(ctx context.Context, id shared.ID) (User, error)
	FindByUsername(ctx context.Context, username Username) (User, error)
}

// Writer is the create/update slice consumed by registration and the
// password-rotation flow (the latter is not yet exposed).
type Writer interface {
	Save(ctx context.Context, u User) (User, error)
}

// StatusUpdater is the narrow port for the status mutation. Splitting
// it from Writer means the UpdateStatus use case can be wired with a
// minimal mock surface (ISP).
type StatusUpdater interface {
	UpdateStatus(ctx context.Context, id shared.ID, status Status) error
}

// AvatarUpdater is the narrow port for the avatar pointer mutation.
type AvatarUpdater interface {
	UpdateAvatar(ctx context.Context, id shared.ID, avatarKey string) error
}
