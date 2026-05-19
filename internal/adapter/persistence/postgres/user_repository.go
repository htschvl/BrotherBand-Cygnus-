package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
)

// UserRepository implements user.Reader, user.Writer,
// user.StatusUpdater and user.AvatarUpdater. It is one struct per
// aggregate, satisfying the multiple narrow ports declared in
// `domain/user/repository.go`.
type UserRepository struct {
	db DBTX
}

// NewUserRepository takes any DBTX (the pool in production, a
// transaction in tests) so this exact same code is exercised end to
// end during integration testing.
func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

// Compile-time interface checks.
var (
	_ user.Reader        = (*UserRepository)(nil)
	_ user.Writer        = (*UserRepository)(nil)
	_ user.StatusUpdater = (*UserRepository)(nil)
	_ user.AvatarUpdater = (*UserRepository)(nil)
)

const (
	userColumns = `id, username, password_hash, birthdate, secret, status, favorites, avatar_key, registered_at`
)

// Save inserts the user. A unique-violation on (username) is
// translated to user.ErrUsernameAlreadyTaken so the use case never
// sees a raw pg error.
func (r *UserRepository) Save(ctx context.Context, u user.User) (user.User, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, birthdate, secret, status, favorites)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+userColumns,
		u.Username().String(),
		u.PasswordHash().String(),
		u.Birthdate().Time(),
		u.Secret().String(),
		u.Status().String(),
		u.Favorites().Values(),
	)
	got, err := scanUser(row)
	if err != nil {
		if pgErrorMatches(err, pgUniqueViolation) {
			return user.User{}, user.ErrUsernameAlreadyTaken
		}
		return user.User{}, fmt.Errorf("postgres: save user: %w", err)
	}
	return got, nil
}

// FindByID returns the user with the given ID, or user.ErrNotFound.
func (r *UserRepository) FindByID(ctx context.Context, id shared.ID) (user.User, error) {
	row := r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id.UUID())
	got, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return user.User{}, user.ErrNotFound
		}
		return user.User{}, fmt.Errorf("postgres: find user by id: %w", err)
	}
	return got, nil
}

// FindByUsername is the case-insensitive lookup used at login. The
// `username` column is CITEXT so the comparison is index-served.
func (r *UserRepository) FindByUsername(ctx context.Context, username user.Username) (user.User, error) {
	row := r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE username = $1`, username.String())
	got, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return user.User{}, user.ErrNotFound
		}
		return user.User{}, fmt.Errorf("postgres: find user by username: %w", err)
	}
	return got, nil
}

// UpdateStatus updates only the status column. The repository does
// not return the updated row because the use case has no need for it.
func (r *UserRepository) UpdateStatus(ctx context.Context, id shared.ID, status user.Status) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET status = $2 WHERE id = $1`, id.UUID(), status.String())
	if err != nil {
		return fmt.Errorf("postgres: update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return user.ErrNotFound
	}
	return nil
}

// UpdateAvatar updates only the avatar_key column.
func (r *UserRepository) UpdateAvatar(ctx context.Context, id shared.ID, avatarKey string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET avatar_key = $2 WHERE id = $1`, id.UUID(), avatarKey)
	if err != nil {
		return fmt.Errorf("postgres: update avatar: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return user.ErrNotFound
	}
	return nil
}

// scanUser is the single point where database columns become a
// rehydrated domain entity. Every repository should call into one of
// these helpers — never inline the column ordering.
func scanUser(row pgxRow) (user.User, error) {
	var (
		id           uuid.UUID
		username     string
		passwordHash string
		birthdate    time.Time
		secret       string
		status       string
		favorites    []string
		avatarKey    *string
		registeredAt time.Time
	)
	if err := row.Scan(&id, &username, &passwordHash, &birthdate, &secret, &status, &favorites, &avatarKey, &registeredAt); err != nil {
		return user.User{}, err
	}
	un, err := user.NewUsername(username)
	if err != nil {
		// A row stored with an invalid username is a corruption
		// signal — the schema already enforces non-empty.
		return user.User{}, errors.Join(user.ErrInvalidUsername, err)
	}
	ph, err := user.NewPasswordHash(passwordHash)
	if err != nil {
		return user.User{}, err
	}
	bd := user.BirthdateFromTime(birthdate)
	sc, err := user.NewSecret(secret)
	if err != nil {
		return user.User{}, err
	}
	st, err := user.NewStatus(status)
	if err != nil {
		return user.User{}, err
	}
	favs, err := user.NewFavorites(favorites)
	if err != nil {
		return user.User{}, err
	}
	idVal, err := shared.ParseID(id.String())
	if err != nil {
		return user.User{}, err
	}
	return user.Rehydrate(idVal, un, ph, bd, sc, st, favs, avatarKey, registeredAt), nil
}

// pgxRow is the minimal subset of pgx.Row used by scan helpers. Both
// pgx.Row and pgx.Rows satisfy it; declaring it locally keeps the
// scan helpers usable in either context.
type pgxRow interface {
	Scan(dest ...any) error
}
