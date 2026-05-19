package brotherband

import (
	"context"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// RequestRepository persists brotherband requests. Splitting it from
// the brotherhood repository keeps each port aligned with the use
// case that needs it (ISP).
type RequestRepository interface {
	Save(ctx context.Context, r Request) (Request, error)
	FindByID(ctx context.Context, id shared.ID) (Request, error)
	Delete(ctx context.Context, id shared.ID) error
	ListReceived(ctx context.Context, userID shared.ID) ([]ReceivedRequest, error)
	ListSent(ctx context.Context, userID shared.ID) ([]SentRequest, error)
}

// BrotherhoodRepository persists confirmed brotherhoods.
type BrotherhoodRepository interface {
	Save(ctx context.Context, b Brotherhood) error
	Delete(ctx context.Context, a, b shared.ID) error
	Exists(ctx context.Context, a, b shared.ID) (bool, error)
	ListBrothers(ctx context.Context, userID shared.ID) ([]Brother, error)
}

// ReceivedRequest decorates a Request with the requester's username
// because the recipient-facing list always needs both. Read models
// living next to the repository keep the use case free of join logic.
type ReceivedRequest struct {
	Request
	RequesterUsername string
}

// SentRequest is the symmetric read model for the sender's view.
type SentRequest struct {
	Request
	RecipientUsername string
}

// Brother is the join row used by the brothers list.
type Brother struct {
	ID               shared.ID
	Username         string
	Status           string
	Favorites        []string
	AvatarKey        *string
	RegisteredAt     time.Time
	BecameBrothersAt time.Time
}
