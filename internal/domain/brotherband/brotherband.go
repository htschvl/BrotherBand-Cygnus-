// Package brotherband is the aggregate that models the trust
// relationship at the heart of BrotherBand: a pending Request that
// either becomes a confirmed Brotherhood or is denied.
//
// The relationship is symmetric, so the persistent representation is
// canonicalised by ordering the two participant IDs. Domain code
// works with unordered pairs; the canonical form lives in the
// repository implementation.
package brotherband

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// Request is a pending invitation from one user to another.
type Request struct {
	id          shared.ID
	requesterID shared.ID
	recipientID shared.ID
	createdAt   time.Time
}

// NewRequest constructs a fresh request and validates the
// "no self-request" invariant. The repository enforces uniqueness on
// (requester, recipient) at write time and returns ErrRequestExists.
func NewRequest(requesterID, recipientID shared.ID, now time.Time) (Request, error) {
	if requesterID.Equals(recipientID) {
		return Request{}, ErrSelfRequest
	}
	return Request{
		id:          shared.NewID(),
		requesterID: requesterID,
		recipientID: recipientID,
		createdAt:   now,
	}, nil
}

// Rehydrate is the adapter-only constructor.
func RehydrateRequest(id, requesterID, recipientID shared.ID, createdAt time.Time) Request {
	return Request{
		id:          id,
		requesterID: requesterID,
		recipientID: recipientID,
		createdAt:   createdAt,
	}
}

func (r Request) ID() shared.ID          { return r.id }
func (r Request) RequesterID() shared.ID { return r.requesterID }
func (r Request) RecipientID() shared.ID { return r.recipientID }
func (r Request) CreatedAt() time.Time   { return r.createdAt }

// Brotherhood is the confirmed, symmetric relationship between two
// users. The constructor canonicalises the pair so equality is
// position-independent.
type Brotherhood struct {
	userA            shared.ID
	userB            shared.ID
	becameBrothersAt time.Time
}

// NewBrotherhood promotes a pair of users to brothers. The caller is
// expected to have just verified an accepted request.
func NewBrotherhood(a, b shared.ID, now time.Time) (Brotherhood, error) {
	if a.Equals(b) {
		return Brotherhood{}, ErrSelfRequest
	}
	return Brotherhood{userA: a, userB: b, becameBrothersAt: now}, nil
}

// RehydrateBrotherhood is the adapter-only constructor.
func RehydrateBrotherhood(a, b shared.ID, becameBrothersAt time.Time) Brotherhood {
	return Brotherhood{userA: a, userB: b, becameBrothersAt: becameBrothersAt}
}

func (b Brotherhood) UserA() shared.ID            { return b.userA }
func (b Brotherhood) UserB() shared.ID            { return b.userB }
func (b Brotherhood) BecameBrothersAt() time.Time { return b.becameBrothersAt }

// Includes reports whether the given user is one of the two members.
func (b Brotherhood) Includes(id shared.ID) bool {
	return b.userA.Equals(id) || b.userB.Equals(id)
}

// Other returns the partner of `id` in this brotherhood. Calling
// `Other` with an id that is not a member returns the zero ID; the
// caller is expected to gate this with `Includes`.
func (b Brotherhood) Other(id shared.ID) shared.ID {
	switch {
	case b.userA.Equals(id):
		return b.userB
	case b.userB.Equals(id):
		return b.userA
	default:
		return shared.ID{}
	}
}
