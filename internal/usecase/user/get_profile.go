package user

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

const componentGetProfile = "usecase.user.get_profile"

// GetProfile reads the authenticated user's full profile view.
type GetProfile struct {
	reader       user.Reader
	avatarLookup port.AvatarURLResolver
}

// NewGetProfile wires the read-only profile lookup.
func NewGetProfile(reader user.Reader, avatarLookup port.AvatarURLResolver) *GetProfile {
	return &GetProfile{reader: reader, avatarLookup: avatarLookup}
}

func (uc *GetProfile) Execute(ctx context.Context, id shared.ID) (ProfileView, error) {
	log := logging.FromContext(ctx).With(logging.Component(componentGetProfile), logging.UserID(id))

	if err := ctx.Err(); err != nil {
		return ProfileView{}, fmt.Errorf("get_profile: context cancelled: %w", err)
	}

	u, err := uc.reader.FindByID(ctx, id)
	if err != nil {
		// A 404 on /v1/me means the JWT references a deleted user — a
		// rare but operationally meaningful event, hence WARN not ERROR.
		level := slog.LevelError
		if errors.Is(err, user.ErrNotFound) {
			level = slog.LevelWarn
		}
		log.LogAttrs(ctx, level, "get_profile: lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return ProfileView{}, err
	}
	return toProfileView(u, uc.avatarLookup), nil
}
