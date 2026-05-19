package user

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

const componentUpdateStatus = "usecase.user.update_status"

// UpdateStatus replaces the authenticated user's status with a new
// validated value.
type UpdateStatus struct {
	statusUpdater user.StatusUpdater
}

// NewUpdateStatus wires the use case against the narrow
// StatusUpdater port.
func NewUpdateStatus(statusUpdater user.StatusUpdater) *UpdateStatus {
	return &UpdateStatus{statusUpdater: statusUpdater}
}

func (uc *UpdateStatus) Execute(ctx context.Context, in UpdateStatusInput) error {
	log := logging.FromContext(ctx).With(logging.Component(componentUpdateStatus), logging.UserID(in.UserID))

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("update_status: context cancelled: %w", err)
	}

	status, err := user.NewStatus(in.Status)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "update_status rejected: invalid value",
			slog.String(logging.AttrError, err.Error()),
		)
		return err
	}
	if err := uc.statusUpdater.UpdateStatus(ctx, in.UserID, status); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "update_status: repository failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return fmt.Errorf("update_status: %w", err)
	}
	log.LogAttrs(ctx, slog.LevelInfo, "status updated",
		slog.String(logging.AttrEvent, "user.status_updated"),
	)
	return nil
}
