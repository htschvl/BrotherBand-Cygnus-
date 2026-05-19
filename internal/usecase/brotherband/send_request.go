package brotherband

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

const componentSend = "usecase.brotherband.send"

// SendRequest creates a pending brotherband request. The use case
// guards against two product invariants:
//
//  1. The two users cannot already be brothers (returns ErrAlreadyBrothers).
//  2. The requested target must exist (returns user.ErrNotFound).
//
// Uniqueness over the (requester, recipient) pair is enforced at the
// DB; the adapter translates the unique-violation into
// brotherband.ErrRequestExists.
type SendRequest struct {
	requests    brotherband.RequestRepository
	brotherhood brotherband.BrotherhoodRepository
	users       user.Reader
	clock       port.Clock
}

// NewSendRequest wires the use case with the narrowest ports it
// needs.
func NewSendRequest(
	requests brotherband.RequestRepository,
	brotherhood brotherband.BrotherhoodRepository,
	users user.Reader,
	clock port.Clock,
) *SendRequest {
	return &SendRequest{requests: requests, brotherhood: brotherhood, users: users, clock: clock}
}

func (uc *SendRequest) Execute(ctx context.Context, in SendRequestInput) (SendRequestOutput, error) {
	log := logging.FromContext(ctx).With(
		logging.Component(componentSend),
		logging.UserID(in.RequesterID),
		logging.TargetUserID(in.RecipientID),
	)

	if err := ctx.Err(); err != nil {
		return SendRequestOutput{}, fmt.Errorf("send_request: context cancelled: %w", err)
	}
	if in.RequesterID.Equals(in.RecipientID) {
		log.LogAttrs(ctx, slog.LevelDebug, "send_request rejected: self-request")
		return SendRequestOutput{}, brotherband.ErrSelfRequest
	}

	requester, err := uc.users.FindByID(ctx, in.RequesterID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "send_request: requester lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return SendRequestOutput{}, fmt.Errorf("send_request: requester: %w", err)
	}
	recipient, err := uc.users.FindByID(ctx, in.RecipientID)
	if err != nil {
		level := slog.LevelError
		if errors.Is(err, user.ErrNotFound) {
			level = slog.LevelInfo
		}
		log.LogAttrs(ctx, level, "send_request: recipient lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return SendRequestOutput{}, err
	}

	alreadyBrothers, err := uc.brotherhood.Exists(ctx, in.RequesterID, in.RecipientID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "send_request: brotherhood probe failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return SendRequestOutput{}, fmt.Errorf("send_request: probe: %w", err)
	}
	if alreadyBrothers {
		log.LogAttrs(ctx, slog.LevelInfo, "send_request rejected: already brothers")
		return SendRequestOutput{}, brotherband.ErrAlreadyBrothers
	}

	req, err := brotherband.NewRequest(in.RequesterID, in.RecipientID, uc.clock.Now())
	if err != nil {
		// NewRequest only fails on self-request, which we've already
		// gated. Treat anything else as a programming error.
		log.LogAttrs(ctx, slog.LevelError, "send_request: domain constructor failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return SendRequestOutput{}, err
	}
	saved, err := uc.requests.Save(ctx, req)
	if err != nil {
		level := slog.LevelError
		if errors.Is(err, brotherband.ErrRequestExists) {
			level = slog.LevelInfo
		}
		log.LogAttrs(ctx, level, "send_request: save failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return SendRequestOutput{}, err
	}

	log.LogAttrs(ctx, slog.LevelInfo, "brotherband request sent",
		slog.String(logging.AttrRequestUUID, saved.ID().String()),
		slog.String(logging.AttrEvent, "brotherband.request_sent"),
	)
	return SendRequestOutput{
		ID:                saved.ID(),
		RequesterID:       saved.RequesterID(),
		RecipientID:       saved.RecipientID(),
		RequesterUsername: requester.Username().String(),
		RecipientUsername: recipient.Username().String(),
		CreatedAt:         saved.CreatedAt(),
	}, nil
}
