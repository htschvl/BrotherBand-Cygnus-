package user

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

const componentUpdateAvatar = "usecase.user.update_avatar"

// UpdateAvatar promotes a previously uploaded pending media key into
// the user's permanent avatar slot. The use case is the seam between
// the storage adapter (which physically moves the object out of the
// `pending/` prefix) and the user repository (which persists the new
// pointer).
type UpdateAvatar struct {
	avatarUpdater user.AvatarUpdater
	imageStore    media.ImageStore
}

// NewUpdateAvatar wires the use case across the storage adapter
// (object promotion) and the user repository (pointer persistence).
func NewUpdateAvatar(avatarUpdater user.AvatarUpdater, imageStore media.ImageStore) *UpdateAvatar {
	return &UpdateAvatar{avatarUpdater: avatarUpdater, imageStore: imageStore}
}

func (uc *UpdateAvatar) Execute(ctx context.Context, in UpdateAvatarInput) error {
	log := logging.FromContext(ctx).With(
		logging.Component(componentUpdateAvatar),
		logging.UserID(in.UserID),
	)

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("update_avatar: context cancelled: %w", err)
	}

	expectedPrefix := "pending/" + in.UserID.String() + "/"
	if !strings.HasPrefix(in.MediaKey, expectedPrefix) {
		// The media key MUST belong to the actor — both because we
		// derive the destination path from it and because the OWASP
		// "broken access control" risk demands it.
		log.LogAttrs(ctx, slog.LevelWarn, "update_avatar rejected: media key not owned by caller",
			slog.String(logging.AttrMediaKey, in.MediaKey),
		)
		return media.ErrPromotionFailed
	}
	finalKey := "avatars/" + in.UserID.String() + "/" + strings.TrimPrefix(in.MediaKey, expectedPrefix)

	if err := uc.imageStore.PromoteFromPending(ctx, in.MediaKey, finalKey); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "update_avatar: promotion failed",
			slog.String(logging.AttrMediaKey, in.MediaKey),
			slog.String(logging.AttrError, err.Error()),
		)
		return fmt.Errorf("update_avatar: promote: %w", err)
	}
	if err := uc.avatarUpdater.UpdateAvatar(ctx, in.UserID, finalKey); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "update_avatar: repository failed",
			slog.String(logging.AttrMediaKey, finalKey),
			slog.String(logging.AttrError, err.Error()),
		)
		return fmt.Errorf("update_avatar: persist: %w", err)
	}
	log.LogAttrs(ctx, slog.LevelInfo, "avatar updated",
		slog.String(logging.AttrMediaKey, finalKey),
		slog.String(logging.AttrEvent, "user.avatar_updated"),
	)
	return nil
}
