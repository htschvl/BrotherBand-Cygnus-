package message

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// Conversation is the container that messages belong to. In Cygnus,
// every conversation is between exactly two users (a brother pair),
// but the schema and the aggregate are written so a future
// group-conversation feature does not require a migration.
type Conversation struct {
	id        shared.ID
	createdAt time.Time
}

// NewConversation constructs a fresh conversation.
func NewConversation(now time.Time) Conversation {
	return Conversation{id: shared.NewID(), createdAt: now}
}

// RehydrateConversation is the adapter-only constructor.
func RehydrateConversation(id shared.ID, createdAt time.Time) Conversation {
	return Conversation{id: id, createdAt: createdAt}
}

func (c Conversation) ID() shared.ID        { return c.id }
func (c Conversation) CreatedAt() time.Time { return c.createdAt }
