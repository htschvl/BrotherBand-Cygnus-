package brotherband

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

const componentDeny = "usecase.brotherband.deny"

// DenyRequest discards a pending brotherband request. Only the
// recipient may deny.
type DenyRequest struct {
	requests brotherband.RequestRepository
}

// NewDenyRequest wires the use case against the request repository.
func NewDenyRequest(requests brotherband.RequestRepository) *DenyRequest {
	return &DenyRequest{requests: requests}
}

func (uc *DenyRequest) Execute(ctx context.Context, in DenyInput) error {
	log := logging.FromContext(ctx).With(
		logging.Component(componentDeny),
		logging.UserID(in.UserID),
		slog.String(logging.AttrRequestUUID, in.RequestID.String()),
	)

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("deny: context cancelled: %w", err)
	}

	req, err := uc.requests.FindByID(ctx, in.RequestID)
	if err != nil {
		level := slog.LevelError
		if errors.Is(err, brotherband.ErrRequestNotFound) {
			level = slog.LevelInfo
		}
		log.LogAttrs(ctx, level, "deny: request lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return err
	}
	if !req.RecipientID().Equals(in.UserID) {
		log.LogAttrs(ctx, slog.LevelWarn, "deny rejected: caller is not the recipient",
			logging.TargetUserID(req.RecipientID()),
		)
		return brotherband.ErrNotRecipient
	}
	if err := uc.requests.Delete(ctx, req.ID()); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "deny: delete failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return fmt.Errorf("deny: %w", err)
	}
	log.LogAttrs(ctx, slog.LevelInfo, "brotherband request denied",
		slog.String(logging.AttrEvent, "brotherband.denied"),
	)
	return nil
}
