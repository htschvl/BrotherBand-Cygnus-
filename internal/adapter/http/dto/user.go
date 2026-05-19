// Package dto holds the HTTP-shaped request and response structs.
// Two adapters, two shapes: the use case has its own typed DTOs in
// `usecase/<aggregate>/dto.go`; this package speaks JSON.
//
// Conversion functions (toX / fromX) live next to the DTOs so the
// handler reads a request, converts to a use-case input, and
// reciprocally converts the use-case output back to a response DTO.
package dto

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	usecaseuser "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/user"
)

// RegisterRequest is the JSON body of POST /v1/auth/register.
type RegisterRequest struct {
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	Birthdate string   `json:"birthdate"`
	Secret    string   `json:"secret"`
	Status    string   `json:"status"`
	Favorites []string `json:"favorites"`
}

func (r RegisterRequest) ToUseCase() usecaseuser.RegisterUserInput {
	return usecaseuser.RegisterUserInput{
		Username:  r.Username,
		Password:  r.Password,
		Birthdate: r.Birthdate,
		Secret:    r.Secret,
		Status:    r.Status,
		Favorites: r.Favorites,
	}
}

// LoginRequest is the JSON body of POST /v1/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r LoginRequest) ToUseCase() usecaseuser.LoginUserInput {
	return usecaseuser.LoginUserInput{Username: r.Username, Password: r.Password}
}

// UpdateStatusRequest is the JSON body of PATCH /v1/me/status.
type UpdateStatusRequest struct {
	Status string `json:"status"`
}

// UpdateAvatarRequest is the JSON body of PATCH /v1/me/avatar.
type UpdateAvatarRequest struct {
	MediaKey string `json:"mediaKey"`
}

// UserProfileResponse is the shape returned for /v1/me, /v1/auth/login,
// and /v1/auth/register.
type UserProfileResponse struct {
	ID           shared.ID `json:"id"`
	Username     string    `json:"username"`
	Status       string    `json:"status"`
	Favorites    []string  `json:"favorites"`
	AvatarURL    *string   `json:"avatarUrl,omitempty"`
	RegisteredAt time.Time `json:"registeredAt"`
}

// ProfileFromUseCase maps the use-case ProfileView to the wire response.
func ProfileFromUseCase(p usecaseuser.ProfileView) UserProfileResponse {
	return UserProfileResponse{
		ID:           p.ID,
		Username:     p.Username,
		Status:       p.Status,
		Favorites:    p.Favorites,
		AvatarURL:    p.AvatarURL,
		RegisteredAt: p.RegisteredAt,
	}
}
