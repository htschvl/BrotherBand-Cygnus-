package brotherband

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

const componentListBrothers = "usecase.brotherband.list_brothers"

// ListBrothers returns all confirmed brothers for the authenticated
// user. The favorites array is included on each entry because the
// frontend uses it to render the brother card without an extra round
// trip.
type ListBrothers struct {
	brotherhood brotherband.BrotherhoodRepository
	avatars     port.AvatarURLResolver
}

// NewListBrothers wires the confirmed-brothers list.
func NewListBrothers(brotherhood brotherband.BrotherhoodRepository, avatars port.AvatarURLResolver) *ListBrothers {
	return &ListBrothers{brotherhood: brotherhood, avatars: avatars}
}

func (uc *ListBrothers) Execute(ctx context.Context, userID shared.ID) ([]BrotherView, error) {
	log := logging.FromContext(ctx).With(logging.Component(componentListBrothers), logging.UserID(userID))

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list_brothers: context cancelled: %w", err)
	}

	rows, err := uc.brotherhood.ListBrothers(ctx, userID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "list_brothers: repository failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return nil, fmt.Errorf("list_brothers: %w", err)
	}
	out := make([]BrotherView, 0, len(rows))
	for _, b := range rows {
		var avatarURL *string
		if b.AvatarKey != nil && uc.avatars != nil {
			u := uc.avatars.PublicURL(*b.AvatarKey)
			avatarURL = &u
		}
		out = append(out, BrotherView{
			ID:               b.ID,
			Username:         b.Username,
			Status:           b.Status,
			Favorites:        b.Favorites,
			AvatarURL:        avatarURL,
			BecameBrothersAt: b.BecameBrothersAt,
		})
	}
	log.LogAttrs(ctx, slog.LevelDebug, "brothers listed", slog.Int("count", len(out)))
	return out, nil
}
