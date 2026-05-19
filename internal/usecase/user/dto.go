// Package user contains one struct per use case in the user
// aggregate. Each use case exposes Input/Output value types here so
// the HTTP adapter does not have to import the domain entity types
// to call them.
package user

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

// ProfileView is the projection returned by the read-only use cases.
// It includes the avatar's public URL (computed by the adapter, not
// stored) and never the password hash or the secret.
type ProfileView struct {
	ID           shared.ID
	Username     string
	Status       string
	Favorites    []string
	AvatarURL    *string
	RegisteredAt time.Time
}

// Session is the output of registration and login: the user view and
// the freshly minted session + CSRF tokens, ready for the HTTP
// adapter to write as cookies.
type Session struct {
	Profile   ProfileView
	Token     port.IssuedToken
	CSRFToken string
}

// RegisterUserInput is the input to RegisterUser.Execute.
type RegisterUserInput struct {
	Username  string
	Password  string
	Birthdate string
	Secret    string
	Status    string
	Favorites []string
}

// LoginUserInput is the input to LoginUser.Execute.
type LoginUserInput struct {
	Username string
	Password string
}

// UpdateStatusInput is the input to UpdateStatus.Execute.
type UpdateStatusInput struct {
	UserID shared.ID
	Status string
}

// UpdateAvatarInput is the input to UpdateAvatar.Execute.
type UpdateAvatarInput struct {
	UserID   shared.ID
	MediaKey string
}
