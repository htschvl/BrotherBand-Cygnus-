package brotherband

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

const componentGetBrother = "usecase.brotherband.get_brother"

// GetBrother returns the public profile of one specific brother. It
// is the read-side equivalent of ListBrothers for a single ID and
// rejects callers who are not actually brothers.
type GetBrother struct {
	brotherhood brotherband.BrotherhoodRepository
	users       user.Reader
	avatars     port.AvatarURLResolver
}

// NewGetBrother wires the single-brother read model.
func NewGetBrother(brotherhood brotherband.BrotherhoodRepository, users user.Reader, avatars port.AvatarURLResolver) *GetBrother {
	return &GetBrother{brotherhood: brotherhood, users: users, avatars: avatars}
}

func (uc *GetBrother) Execute(ctx context.Context, userID, brotherID shared.ID) (BrotherView, error) {
	log := logging.FromContext(ctx).With(
		logging.Component(componentGetBrother),
		logging.UserID(userID),
		logging.TargetUserID(brotherID),
	)

	if err := ctx.Err(); err != nil {
		return BrotherView{}, fmt.Errorf("get_brother: context cancelled: %w", err)
	}

	exists, err := uc.brotherhood.Exists(ctx, userID, brotherID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "get_brother: probe failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return BrotherView{}, fmt.Errorf("get_brother: probe: %w", err)
	}
	if !exists {
		log.LogAttrs(ctx, slog.LevelInfo, "get_brother rejected: not brothers")
		return BrotherView{}, brotherband.ErrNotABrother
	}
	u, err := uc.users.FindByID(ctx, brotherID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "get_brother: user lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return BrotherView{}, fmt.Errorf("get_brother: lookup: %w", err)
	}
	var avatarURL *string
	if u.AvatarKey() != nil && uc.avatars != nil {
		v := uc.avatars.PublicURL(*u.AvatarKey())
		avatarURL = &v
	}
	return BrotherView{
		ID:        u.ID(),
		Username:  u.Username().String(),
		Status:    u.Status().String(),
		Favorites: u.Favorites().Values(),
		AvatarURL: avatarURL,
	}, nil
}
