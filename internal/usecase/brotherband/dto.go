// Package brotherband contains the use cases that drive the
// brotherband-request lifecycle: send, accept, deny, list, cut.
//
// The "secret reveal" on acceptance is a product-level invariant
// — the accepter must learn the requester's secret exactly once and
// the server must never return it again. The use case enforces this
// by returning it in the AcceptOutput and never on subsequent reads.
package brotherband

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// SendRequestInput is the input to SendRequest.Execute.
type SendRequestInput struct {
	RequesterID shared.ID
	RecipientID shared.ID
}

// SendRequestOutput is returned to the HTTP adapter for the 201
// response body.
type SendRequestOutput struct {
	ID                shared.ID
	RequesterID       shared.ID
	RecipientID       shared.ID
	RequesterUsername string
	RecipientUsername string
	CreatedAt         time.Time
}

// AcceptInput is the input to AcceptRequest.Execute.
type AcceptInput struct {
	UserID    shared.ID
	RequestID shared.ID
}

// AcceptOutput is the result of accepting: a fresh brother view plus
// the one-shot reveal of the requester's secret.
type AcceptOutput struct {
	Brother         BrotherView
	RequesterSecret string
}

// DenyInput is the input to DenyRequest.Execute.
type DenyInput struct {
	UserID    shared.ID
	RequestID shared.ID
}

// CutInput is the input to CutBrotherband.Execute.
type CutInput struct {
	UserID    shared.ID
	BrotherID shared.ID
}

// BrotherView is the projection of a confirmed brother used by the
// list and the accept responses.
type BrotherView struct {
	ID               shared.ID
	Username         string
	Status           string
	Favorites        []string
	AvatarURL        *string
	BecameBrothersAt time.Time
}

// RequestView mirrors the on-the-wire shape of a request and is
// produced by the list use case.
type RequestView struct {
	ID                shared.ID
	RequesterID       shared.ID
	RecipientID       shared.ID
	RequesterUsername string
	RecipientUsername string
	CreatedAt         time.Time
}

// ListRequestsInput is the input to ListRequests.Execute.
type ListRequestsInput struct {
	UserID    shared.ID
	Direction string // "received" | "sent" | "all" (empty = all)
}

// ListRequestsOutput packages both directions; either may be nil
// when filtered out.
type ListRequestsOutput struct {
	Received []RequestView
	Sent     []RequestView
}
