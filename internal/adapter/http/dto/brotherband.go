package dto

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	usecasebb "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/brotherband"
)

// BrotherSummaryResponse is the compact brother projection used in lists.
type BrotherSummaryResponse struct {
	ID               shared.ID  `json:"id"`
	Username         string     `json:"username"`
	Status           string     `json:"status"`
	AvatarURL        *string    `json:"avatarUrl,omitempty"`
	BecameBrothersAt *time.Time `json:"becameBrothersAt,omitempty"`
}

// BrotherProfileResponse is a single brother's full public profile.
type BrotherProfileResponse struct {
	BrotherSummaryResponse
	Favorites []string `json:"favorites"`
}

// BrotherSummaryFromUseCase maps a use-case BrotherView to the wire summary.
func BrotherSummaryFromUseCase(b usecasebb.BrotherView) BrotherSummaryResponse {
	var ts *time.Time
	if !b.BecameBrothersAt.IsZero() {
		v := b.BecameBrothersAt
		ts = &v
	}
	return BrotherSummaryResponse{
		ID:               b.ID,
		Username:         b.Username,
		Status:           b.Status,
		AvatarURL:        b.AvatarURL,
		BecameBrothersAt: ts,
	}
}

// BrothersListResponse is the GET /v1/brothers payload.
type BrothersListResponse struct {
	Brothers []BrotherSummaryResponse `json:"brothers"`
}

// BrotherbandRequestResponse is a single pending request on the wire.
type BrotherbandRequestResponse struct {
	ID                shared.ID `json:"id"`
	RequesterID       shared.ID `json:"requesterId"`
	RecipientID       shared.ID `json:"recipientId"`
	RequesterUsername string    `json:"requesterUsername,omitempty"`
	RecipientUsername string    `json:"recipientUsername,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
}

// BrotherbandRequestListResponse splits pending requests by direction.
type BrotherbandRequestListResponse struct {
	Received []BrotherbandRequestResponse `json:"received"`
	Sent     []BrotherbandRequestResponse `json:"sent"`
}

// BrotherbandAcceptedResponse carries the new brother plus the one-shot secret reveal.
type BrotherbandAcceptedResponse struct {
	Brother         BrotherSummaryResponse `json:"brother"`
	RequesterSecret string                 `json:"requesterSecret"`
}

// RequestViewToResponse maps a use-case RequestView to the wire shape.
func RequestViewToResponse(v usecasebb.RequestView) BrotherbandRequestResponse {
	return BrotherbandRequestResponse{
		ID:                v.ID,
		RequesterID:       v.RequesterID,
		RecipientID:       v.RecipientID,
		RequesterUsername: v.RequesterUsername,
		RecipientUsername: v.RecipientUsername,
		CreatedAt:         v.CreatedAt,
	}
}
