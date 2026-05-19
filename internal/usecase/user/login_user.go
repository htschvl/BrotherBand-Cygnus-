package user

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

const componentLogin = "usecase.user.login"

// LoginUser authenticates a username/password pair and issues a
// fresh session + CSRF token. All failure modes — unknown user,
// wrong password, malformed input — collapse to ErrInvalidCredentials
// to avoid leaking user-existence through error differences.
//
// The error response is uniform; the *log* is not. Internal logs
// distinguish "no such user" from "wrong password" because the
// signal is operationally useful (sudden burst of one or the other
// flags an enumeration attack vs. a credential-stuffing attack).
type LoginUser struct {
	reader       user.Reader
	hasher       port.PasswordHasher
	tokenIssuer  port.TokenIssuer
	csrfMinter   port.CSRFMinter
	clock        port.Clock
	avatarLookup port.AvatarURLResolver
}

// NewLoginUser wires the use case with the narrowest ports it needs.
// The returned value is ready to use.
func NewLoginUser(
	reader user.Reader,
	hasher port.PasswordHasher,
	tokenIssuer port.TokenIssuer,
	csrfMinter port.CSRFMinter,
	clock port.Clock,
	avatarLookup port.AvatarURLResolver,
) *LoginUser {
	return &LoginUser{
		reader: reader, hasher: hasher, tokenIssuer: tokenIssuer,
		csrfMinter: csrfMinter, clock: clock, avatarLookup: avatarLookup,
	}
}

func (uc *LoginUser) Execute(ctx context.Context, in LoginUserInput) (Session, error) {
	log := logging.FromContext(ctx).With(logging.Component(componentLogin))

	if err := ctx.Err(); err != nil {
		return Session{}, fmt.Errorf("login: context cancelled: %w", err)
	}

	username, err := user.NewUsername(in.Username)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "login rejected: malformed username",
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, user.ErrInvalidCredentials
	}
	found, err := uc.reader.FindByUsername(ctx, username)
	if err != nil {
		if asNotFound(err) {
			log.LogAttrs(ctx, slog.LevelInfo, "login failed: unknown username",
				slog.String(logging.AttrUsername, username.String()),
				slog.String(logging.AttrEvent, "auth.login.failed"),
				slog.String("reason", "unknown_username"),
			)
			return Session{}, user.ErrInvalidCredentials
		}
		log.LogAttrs(ctx, slog.LevelError, "login: repository error",
			slog.String(logging.AttrUsername, username.String()),
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, fmt.Errorf("login: lookup: %w", err)
	}
	if err := uc.hasher.Verify(found.PasswordHash().String(), in.Password); err != nil {
		log.LogAttrs(ctx, slog.LevelInfo, "login failed: bad password",
			slog.String(logging.AttrUsername, username.String()),
			slog.String(logging.AttrEvent, "auth.login.failed"),
			slog.String("reason", "bad_password"),
		)
		return Session{}, user.ErrInvalidCredentials
	}

	session, err := buildSession(found, uc.tokenIssuer, uc.csrfMinter, uc.clock, uc.avatarLookup)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "login: session issuance failed",
			logging.UserID(found.ID()),
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, err
	}

	log.LogAttrs(ctx, slog.LevelInfo, "user authenticated",
		logging.UserID(found.ID()),
		slog.String(logging.AttrUsername, found.Username().String()),
		slog.String(logging.AttrEvent, "auth.login.succeeded"),
	)
	return session, nil
}
