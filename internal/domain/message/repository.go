package message

import (
	"context"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// MessageReader is the read-only slice consumed by the listing flow.
type MessageReader interface {
	FindByConversation(
		ctx context.Context,
		conversationID shared.ID,
		cursor *Cursor,
		limit int,
	) ([]Message, *Cursor, error)
}

// MessageWriter is the write slice consumed by the send flow.
type MessageWriter interface {
	Save(ctx context.Context, m Message) (Message, error)
	SaveAttachment(ctx context.Context, a Attachment) (Attachment, error)
	FindByID(ctx context.Context, id shared.ID) (Message, error)
}

// ConversationRepository owns conversation lifecycle and participant
// membership.
type ConversationRepository interface {
	Create(ctx context.Context, c Conversation, participants []shared.ID) (Conversation, error)
	FindDirectBetween(ctx context.Context, a, b shared.ID) (Conversation, bool, error)
	IsParticipant(ctx context.Context, conversationID, userID shared.ID) (bool, error)
	UpdateLastRead(ctx context.Context, conversationID, userID shared.ID, at time.Time) error
}
