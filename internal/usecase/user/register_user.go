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

const componentRegister = "usecase.user.register"

// RegisterUser creates a new user account and issues a session in
// one transaction-flavoured step. The repository call is the
// boundary at which uniqueness is enforced — the use case maps the
// repository's conflict back to the canonical domain error.
type RegisterUser struct {
	writer       user.Writer
	reader       user.Reader
	hasher       port.PasswordHasher
	tokenIssuer  port.TokenIssuer
	csrfMinter   port.CSRFMinter
	clock        port.Clock
	avatarLookup port.AvatarURLResolver
}

// NewRegisterUser wires the use case. Each dependency is the
// narrowest port the operation needs.
func NewRegisterUser(
	writer user.Writer,
	reader user.Reader,
	hasher port.PasswordHasher,
	tokenIssuer port.TokenIssuer,
	csrfMinter port.CSRFMinter,
	clock port.Clock,
	avatarLookup port.AvatarURLResolver,
) *RegisterUser {
	return &RegisterUser{
		writer: writer, reader: reader, hasher: hasher,
		tokenIssuer: tokenIssuer, csrfMinter: csrfMinter,
		clock: clock, avatarLookup: avatarLookup,
	}
}

// Execute runs the use case. The order — validate, hash, write,
// issue tokens — keeps the expensive argon2id call from running for
// inputs the validator would have rejected.
//
// Logging contract:
//   - INFO  on success ("user registered")
//   - WARN  on the username-conflict path (a meaningful product event)
//   - DEBUG on validation rejections (high-volume, low-signal)
//   - ERROR on unexpected hash / token / repository failures
func (uc *RegisterUser) Execute(ctx context.Context, in RegisterUserInput) (Session, error) {
	log := logging.FromContext(ctx).With(logging.Component(componentRegister))

	if err := ctx.Err(); err != nil {
		return Session{}, fmt.Errorf("register: context cancelled before validation: %w", err)
	}

	if err := user.ValidateRawPassword(in.Password); err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "register rejected: password",
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, err
	}
	username, err := user.NewUsername(in.Username)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "register rejected: username",
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, err
	}
	birthdate, err := user.NewBirthdate(in.Birthdate)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "register rejected: birthdate",
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, err
	}
	secret, err := user.NewSecret(in.Secret)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "register rejected: secret",
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, err
	}
	status, err := user.NewStatus(in.Status)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "register rejected: status",
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, err
	}
	favorites, err := user.NewFavorites(in.Favorites)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "register rejected: favorites",
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, err
	}

	encoded, err := uc.hasher.Hash(in.Password)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "register: password hashing failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, fmt.Errorf("register: hash password: %w", err)
	}
	passwordHash, err := user.NewPasswordHash(encoded)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "register: hasher returned an empty digest",
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, fmt.Errorf("register: wrap hash: %w", err)
	}

	now := uc.clock.Now()
	candidate := user.New(username, passwordHash, birthdate, secret, status, favorites, now)

	saved, err := uc.writer.Save(ctx, candidate)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrUsernameAlreadyTaken):
			log.LogAttrs(ctx, slog.LevelWarn, "register rejected: username taken",
				slog.String(logging.AttrUsername, username.String()),
			)
		default:
			log.LogAttrs(ctx, slog.LevelError, "register: save failed",
				slog.String(logging.AttrUsername, username.String()),
				slog.String(logging.AttrError, err.Error()),
			)
		}
		return Session{}, err
	}

	session, err := buildSession(saved, uc.tokenIssuer, uc.csrfMinter, uc.clock, uc.avatarLookup)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "register: session issuance failed",
			logging.UserID(saved.ID()),
			slog.String(logging.AttrError, err.Error()),
		)
		return Session{}, err
	}

	log.LogAttrs(ctx, slog.LevelInfo, "user registered",
		logging.UserID(saved.ID()),
		slog.String(logging.AttrUsername, saved.Username().String()),
		slog.String(logging.AttrEvent, "user.registered"),
	)
	return session, nil
}

// buildSession is shared between RegisterUser and LoginUser to keep
// the cookie-issuance shape consistent. Errors here are infrastructure
// failures — the caller logs them with the right context.
func buildSession(
	u user.User,
	issuer port.TokenIssuer,
	csrf port.CSRFMinter,
	clock port.Clock,
	avatars port.AvatarURLResolver,
) (Session, error) {
	token, err := issuer.Issue(u.ID(), clock.Now())
	if err != nil {
		return Session{}, fmt.Errorf("session: issue token: %w", err)
	}
	csrfToken, err := csrf.Mint()
	if err != nil {
		return Session{}, fmt.Errorf("session: mint csrf: %w", err)
	}
	return Session{
		Profile:   toProfileView(u, avatars),
		Token:     token,
		CSRFToken: csrfToken,
	}, nil
}

// toProfileView projects the domain user into the use-case-level
// view. Used by register, login, and getProfile.
func toProfileView(u user.User, avatars port.AvatarURLResolver) ProfileView {
	var avatarURL *string
	if u.AvatarKey() != nil && avatars != nil {
		url := avatars.PublicURL(*u.AvatarKey())
		avatarURL = &url
	}
	return ProfileView{
		ID:           u.ID(),
		Username:     u.Username().String(),
		Status:       u.Status().String(),
		Favorites:    u.Favorites().Values(),
		AvatarURL:    avatarURL,
		RegisteredAt: u.RegisteredAt(),
	}
}

// asNotFound is the small helper used by use cases that need to
// recognise a "no rows" result from the reader without leaking the
// adapter's error type.
func asNotFound(err error) bool {
	return errors.Is(err, user.ErrNotFound) || errors.Is(err, shared.ErrNotFound)
}
