package user

import (
	"errors"
	"fmt"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// All user-specific errors wrap a shared sentinel so the HTTP error
// map can classify them by category without enumerating each one.
var (
	ErrUsernameAlreadyTaken = fmt.Errorf("user: username already taken: %w", shared.ErrConflict)
	ErrInvalidCredentials   = fmt.Errorf("user: invalid credentials: %w", shared.ErrUnauthenticated)
	ErrPasswordTooWeak      = fmt.Errorf("user: password too weak: %w", shared.ErrInvalidInput)
	ErrInvalidUsername      = fmt.Errorf("user: invalid username: %w", shared.ErrInvalidInput)
	ErrInvalidPasswordHash  = fmt.Errorf("user: invalid password hash: %w", shared.ErrInvalidInput)
	ErrInvalidBirthdate     = fmt.Errorf("user: invalid birthdate: %w", shared.ErrInvalidInput)
	ErrInvalidSecret        = fmt.Errorf("user: invalid secret: %w", shared.ErrInvalidInput)
	ErrInvalidStatus        = fmt.Errorf("user: invalid status: %w", shared.ErrInvalidInput)
	ErrInvalidFavorites     = fmt.Errorf("user: favorites must be exactly five non-empty strings: %w", shared.ErrInvalidInput)
	ErrNotFound             = fmt.Errorf("user: not found: %w", shared.ErrNotFound)
)

// IsNotFound is a small helper for adapter code that wants to map a
// "no rows" condition without importing two error sentinels.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
