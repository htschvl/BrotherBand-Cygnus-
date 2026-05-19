package message

import (
	"fmt"
	"strings"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// Message is the message aggregate root.
type Message struct {
	id             shared.ID
	conversationID shared.ID
	senderID       shared.ID
	body           Body
	createdAt      time.Time
	editedAt       *time.Time
	attachments    []Attachment
}

// New constructs a brand-new message. Conversation membership is
// not validated here — it is the use case's responsibility to call
// ConversationRepository.IsParticipant before invoking New.
func New(conversationID, senderID shared.ID, body Body, now time.Time) Message {
	return Message{
		id:             shared.NewID(),
		conversationID: conversationID,
		senderID:       senderID,
		body:           body,
		createdAt:      now,
	}
}

// Rehydrate is the adapter-only constructor.
func Rehydrate(
	id, conversationID, senderID shared.ID,
	body Body,
	createdAt time.Time,
	editedAt *time.Time,
	attachments []Attachment,
) Message {
	return Message{
		id:             id,
		conversationID: conversationID,
		senderID:       senderID,
		body:           body,
		createdAt:      createdAt,
		editedAt:       editedAt,
		attachments:    attachments,
	}
}

func (m Message) ID() shared.ID             { return m.id }
func (m Message) ConversationID() shared.ID { return m.conversationID }
func (m Message) SenderID() shared.ID       { return m.senderID }
func (m Message) Body() Body                { return m.body }
func (m Message) CreatedAt() time.Time      { return m.createdAt }
func (m Message) EditedAt() *time.Time      { return m.editedAt }
func (m Message) Attachments() []Attachment {
	out := make([]Attachment, len(m.attachments))
	copy(out, m.attachments)
	return out
}

// WithAttachments returns a copy of the message with attachments
// replaced — used by repositories after loading the join.
func (m Message) WithAttachments(attachments []Attachment) Message {
	m.attachments = attachments
	return m
}

// Body is the validated message text.
type Body struct{ value string }

// Field name shared by the value-object error reasons and the HTTP DTO.
const FieldBody = "body"

const (
	BodyMinLen = 1
	BodyMaxLen = 4000
)

// NewBody validates the message body. Whitespace is trimmed; an
// all-whitespace body is rejected with a typed ValidationError.
func NewBody(raw string) (Body, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < BodyMinLen || len(trimmed) > BodyMaxLen {
		return Body{}, shared.WrapValidation(
			ErrInvalidBody,
			FieldBody,
			fmt.Sprintf("must be between %d and %d characters after trimming", BodyMinLen, BodyMaxLen),
		)
	}
	return Body{value: trimmed}, nil
}

func (b Body) String() string { return b.value }
