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

const componentAccept = "usecase.brotherband.accept"

// AcceptRequest accepts a pending brotherband request: it deletes
// the request, creates the brotherhood, and returns the requester's
// secret in a one-shot reveal.
//
// Authorisation: only the *recipient* of the request can accept it.
// Any other caller gets brotherband.ErrNotRecipient.
type AcceptRequest struct {
	requests    brotherband.RequestRepository
	brotherhood brotherband.BrotherhoodRepository
	users       user.Reader
	avatars     port.AvatarURLResolver
	clock       port.Clock
}

// NewAcceptRequest wires the use case with the narrowest ports it
// needs.
func NewAcceptRequest(
	requests brotherband.RequestRepository,
	brotherhood brotherband.BrotherhoodRepository,
	users user.Reader,
	avatars port.AvatarURLResolver,
	clock port.Clock,
) *AcceptRequest {
	return &AcceptRequest{
		requests: requests, brotherhood: brotherhood, users: users,
		avatars: avatars, clock: clock,
	}
}

func (uc *AcceptRequest) Execute(ctx context.Context, in AcceptInput) (AcceptOutput, error) {
	log := logging.FromContext(ctx).With(
		logging.Component(componentAccept),
		logging.UserID(in.UserID),
		slog.String(logging.AttrRequestUUID, in.RequestID.String()),
	)

	if err := ctx.Err(); err != nil {
		return AcceptOutput{}, fmt.Errorf("accept: context cancelled: %w", err)
	}

	req, err := uc.requests.FindByID(ctx, in.RequestID)
	if err != nil {
		level := slog.LevelError
		if errors.Is(err, brotherband.ErrRequestNotFound) {
			level = slog.LevelInfo
		}
		log.LogAttrs(ctx, level, "accept: request lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return AcceptOutput{}, err
	}
	if !req.RecipientID().Equals(in.UserID) {
		log.LogAttrs(ctx, slog.LevelWarn, "accept rejected: caller is not the recipient",
			logging.TargetUserID(req.RecipientID()),
		)
		return AcceptOutput{}, brotherband.ErrNotRecipient
	}

	exists, err := uc.brotherhood.Exists(ctx, req.RequesterID(), req.RecipientID())
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "accept: brotherhood probe failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return AcceptOutput{}, fmt.Errorf("accept: probe: %w", err)
	}
	if exists {
		// Defensive: a brotherhood appeared (e.g. via a parallel accept).
		// Delete the stale request and surface the conflict.
		log.LogAttrs(ctx, slog.LevelWarn, "accept: brotherhood already exists; cleaning stale request")
		if delErr := uc.requests.Delete(ctx, req.ID()); delErr != nil &&
			!errors.Is(delErr, brotherband.ErrRequestNotFound) {
			log.LogAttrs(ctx, slog.LevelError, "accept: stale-request delete failed",
				slog.String(logging.AttrError, delErr.Error()),
			)
		}
		return AcceptOutput{}, brotherband.ErrAlreadyBrothers
	}

	requester, err := uc.users.FindByID(ctx, req.RequesterID())
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "accept: requester lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return AcceptOutput{}, fmt.Errorf("accept: requester: %w", err)
	}

	now := uc.clock.Now()
	bond, err := brotherband.NewBrotherhood(req.RequesterID(), req.RecipientID(), now)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "accept: brotherhood constructor failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return AcceptOutput{}, err
	}
	if err := uc.brotherhood.Save(ctx, bond); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "accept: brotherhood save failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return AcceptOutput{}, fmt.Errorf("accept: save brotherhood: %w", err)
	}
	if err := uc.requests.Delete(ctx, req.ID()); err != nil {
		// The brotherhood is saved; the orphaned request is harmless
		// (UI lists pending requests via the same recipient predicate
		// and skips fulfilled ones). Log loudly but don't fail.
		log.LogAttrs(ctx, slog.LevelError, "accept: post-bond request delete failed (orphaned but harmless)",
			slog.String(logging.AttrError, err.Error()),
		)
	}

	var avatarURL *string
	if requester.AvatarKey() != nil && uc.avatars != nil {
		u := uc.avatars.PublicURL(*requester.AvatarKey())
		avatarURL = &u
	}

	log.LogAttrs(ctx, slog.LevelInfo, "brotherband accepted",
		logging.TargetUserID(requester.ID()),
		slog.String(logging.AttrEvent, "brotherband.accepted"),
	)
	return AcceptOutput{
		Brother: BrotherView{
			ID:               requester.ID(),
			Username:         requester.Username().String(),
			Status:           requester.Status().String(),
			Favorites:        requester.Favorites().Values(),
			AvatarURL:        avatarURL,
			BecameBrothersAt: now,
		},
		RequesterSecret: requester.Secret().String(),
	}, nil
}
