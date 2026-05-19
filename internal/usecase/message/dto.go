// Package message contains the message-aggregate use cases:
// SendMessage, ListMessages, AttachMedia, ListConversations.
//
// Conversations are 1:1 in this iteration but the aggregate is
// modelled as N-participant; the use cases call the conversation
// repository to find or create the conversation between two
// brothers, never construct it directly.
package message

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// SendInput is the input to SendMessage.Execute.
type SendInput struct {
	SenderID  shared.ID
	BrotherID shared.ID
	Body      string
}

// AttachInput is the input to AttachMedia.Execute.
type AttachInput struct {
	ActorID   shared.ID
	MessageID shared.ID
	MediaKey  string
}

// ListInput is the input to ListMessages.Execute.
type ListInput struct {
	ActorID   shared.ID
	BrotherID shared.ID
	Cursor    string
	Limit     int
}

// MessageView is the projection returned to the HTTP layer.
type MessageView struct {
	ID             shared.ID
	ConversationID shared.ID
	SenderID       shared.ID
	Body           string
	CreatedAt      time.Time
	EditedAt       *time.Time
	Attachments    []AttachmentView
}

// AttachmentView contains the public URL the client renders.
type AttachmentView struct {
	MediaKey    string
	URL         string
	ContentType string
	SizeBytes   int64
}

// ListOutput is the cursor-paginated page returned by ListMessages.
type ListOutput struct {
	Items      []MessageView
	NextCursor string
}

// ConversationSummary is one row in ListConversations.
type ConversationSummary struct {
	ConversationID     shared.ID
	BrotherID          shared.ID
	BrotherUsername    string
	BrotherStatus      string
	BrotherAvatarURL   *string
	BecameBrothersAt   time.Time
	LastMessageAt      *time.Time
	LastMessagePreview *string
	UnreadCount        int
}
