package message

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

const componentSendMessage = "usecase.message.send"

// SendMessage delivers a message from one brother to another. The
// use case performs three things in order:
//  1. Verify the sender and target are confirmed brothers.
//  2. Find or create the direct conversation between them.
//  3. Append the message.
//
// All persistence calls go through narrow ports; the conversation
// repository is responsible for the find-or-create idempotency.
type SendMessage struct {
	conversations message.ConversationRepository
	messages      message.MessageWriter
	brotherhood   brotherband.BrotherhoodRepository
	clock         port.Clock
	media         media.ImageStore
}

// NewSendMessage wires the use case with the narrowest ports it
// needs.
func NewSendMessage(
	conversations message.ConversationRepository,
	messages message.MessageWriter,
	brotherhood brotherband.BrotherhoodRepository,
	clock port.Clock,
	media media.ImageStore,
) *SendMessage {
	return &SendMessage{
		conversations: conversations, messages: messages,
		brotherhood: brotherhood, clock: clock, media: media,
	}
}

func (uc *SendMessage) Execute(ctx context.Context, in SendInput) (MessageView, error) {
	log := logging.FromContext(ctx).With(
		logging.Component(componentSendMessage),
		logging.UserID(in.SenderID),
		logging.TargetUserID(in.BrotherID),
	)

	if err := ctx.Err(); err != nil {
		return MessageView{}, fmt.Errorf("send_message: context cancelled: %w", err)
	}

	body, err := message.NewBody(in.Body)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "send_message rejected: invalid body",
			slog.String(logging.AttrError, err.Error()),
		)
		return MessageView{}, err
	}
	areBrothers, err := uc.brotherhood.Exists(ctx, in.SenderID, in.BrotherID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "send_message: brotherhood probe failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return MessageView{}, fmt.Errorf("send_message: probe: %w", err)
	}
	if !areBrothers {
		log.LogAttrs(ctx, slog.LevelInfo, "send_message rejected: not brothers")
		return MessageView{}, brotherband.ErrNotABrother
	}

	conv, found, err := uc.conversations.FindDirectBetween(ctx, in.SenderID, in.BrotherID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "send_message: conversation lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return MessageView{}, fmt.Errorf("send_message: find conversation: %w", err)
	}
	if !found {
		conv, err = uc.conversations.Create(
			ctx,
			message.NewConversation(uc.clock.Now()),
			[]shared.ID{in.SenderID, in.BrotherID},
		)
		if err != nil {
			log.LogAttrs(ctx, slog.LevelError, "send_message: conversation create failed",
				slog.String(logging.AttrError, err.Error()),
			)
			return MessageView{}, fmt.Errorf("send_message: create conversation: %w", err)
		}
		log.LogAttrs(ctx, slog.LevelInfo, "conversation created on first message",
			slog.String(logging.AttrConvID, conv.ID().String()),
			slog.String(logging.AttrEvent, "conversation.created"),
		)
	}

	saved, err := uc.messages.Save(ctx, message.New(conv.ID(), in.SenderID, body, uc.clock.Now()))
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "send_message: save failed",
			slog.String(logging.AttrConvID, conv.ID().String()),
			slog.String(logging.AttrError, err.Error()),
		)
		return MessageView{}, fmt.Errorf("send_message: save: %w", err)
	}

	log.LogAttrs(ctx, slog.LevelInfo, "message sent",
		slog.String(logging.AttrConvID, conv.ID().String()),
		slog.String(logging.AttrMessageID, saved.ID().String()),
		slog.String(logging.AttrEvent, "message.sent"),
	)
	return toMessageView(saved, uc.media), nil
}

// toMessageView is shared between send / list / attach so the
// projection logic exists in one place.
func toMessageView(m message.Message, store media.ImageStore) MessageView {
	view := MessageView{
		ID:             m.ID(),
		ConversationID: m.ConversationID(),
		SenderID:       m.SenderID(),
		Body:           m.Body().String(),
		CreatedAt:      m.CreatedAt(),
		EditedAt:       m.EditedAt(),
	}
	for _, a := range m.Attachments() {
		view.Attachments = append(view.Attachments, AttachmentView{
			MediaKey:    a.MediaKey(),
			URL:         store.PublicURL(a.MediaKey()),
			ContentType: a.ContentType(),
			SizeBytes:   a.SizeBytes(),
		})
	}
	return view
}
