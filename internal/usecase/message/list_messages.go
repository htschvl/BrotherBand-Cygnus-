package message

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

const componentListMessages = "usecase.message.list"

// ListMessages returns a cursor-paginated page of messages exchanged
// with the given brother, newest first. The opaque cursor encoding
// is owned by the message domain (Cursor.Encode / DecodeCursor).
type ListMessages struct {
	conversations message.ConversationRepository
	messages      message.MessageReader
	brotherhood   brotherband.BrotherhoodRepository
	media         media.ImageStore
}

// NewListMessages wires the cursor-paginated message reader.
func NewListMessages(
	conversations message.ConversationRepository,
	messages message.MessageReader,
	brotherhood brotherband.BrotherhoodRepository,
	mediaStore media.ImageStore,
) *ListMessages {
	return &ListMessages{
		conversations: conversations, messages: messages,
		brotherhood: brotherhood, media: mediaStore,
	}
}

const (
	defaultListLimit = 50
	maxListLimit     = 100
)

func (uc *ListMessages) Execute(ctx context.Context, in ListInput) (ListOutput, error) {
	log := logging.FromContext(ctx).With(
		logging.Component(componentListMessages),
		logging.UserID(in.ActorID),
		logging.TargetUserID(in.BrotherID),
	)

	if err := ctx.Err(); err != nil {
		return ListOutput{}, fmt.Errorf("list_messages: context cancelled: %w", err)
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	areBrothers, err := uc.brotherhood.Exists(ctx, in.ActorID, in.BrotherID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "list_messages: brotherhood probe failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return ListOutput{}, fmt.Errorf("list_messages: probe: %w", err)
	}
	if !areBrothers {
		log.LogAttrs(ctx, slog.LevelInfo, "list_messages rejected: not brothers")
		return ListOutput{}, brotherband.ErrNotABrother
	}
	conv, found, err := uc.conversations.FindDirectBetween(ctx, in.ActorID, in.BrotherID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "list_messages: conversation lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return ListOutput{}, fmt.Errorf("list_messages: find conversation: %w", err)
	}
	if !found {
		// Brothers who have never exchanged a message: an empty page,
		// not an error.
		return ListOutput{Items: []MessageView{}}, nil
	}

	cursor, err := message.DecodeCursor(in.Cursor)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "list_messages rejected: bad cursor",
			slog.String(logging.AttrError, err.Error()),
		)
		return ListOutput{}, err
	}

	items, next, err := uc.messages.FindByConversation(ctx, conv.ID(), cursor, limit)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "list_messages: page query failed",
			slog.String(logging.AttrConvID, conv.ID().String()),
			slog.String(logging.AttrError, err.Error()),
		)
		return ListOutput{}, fmt.Errorf("list_messages: page: %w", err)
	}

	out := ListOutput{Items: make([]MessageView, 0, len(items))}
	for _, m := range items {
		out.Items = append(out.Items, toMessageView(m, uc.media))
	}
	if next != nil {
		out.NextCursor = next.Encode()
	}
	log.LogAttrs(ctx, slog.LevelDebug, "messages listed",
		slog.String(logging.AttrConvID, conv.ID().String()),
		slog.Int("count", len(out.Items)),
		slog.Bool("has_next", next != nil),
	)
	return out, nil
}
