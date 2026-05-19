package message

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

const componentListConversations = "usecase.message.list_conversations"

// ListConversations returns a row per confirmed brother, including
// the matching conversation if one exists. This shape lets the
// frontend render the conversations list without an N+1 round trip.
type ListConversations struct {
	brotherhood   brotherband.BrotherhoodRepository
	conversations message.ConversationRepository
	avatars       port.AvatarURLResolver
}

// NewListConversations wires the conversation-list projection.
func NewListConversations(
	brotherhood brotherband.BrotherhoodRepository,
	conversations message.ConversationRepository,
	avatars port.AvatarURLResolver,
) *ListConversations {
	return &ListConversations{
		brotherhood: brotherhood, conversations: conversations, avatars: avatars,
	}
}

func (uc *ListConversations) Execute(ctx context.Context, userID shared.ID) ([]ConversationSummary, error) {
	log := logging.FromContext(ctx).With(logging.Component(componentListConversations), logging.UserID(userID))

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list_conversations: context cancelled: %w", err)
	}

	brothers, err := uc.brotherhood.ListBrothers(ctx, userID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "list_conversations: brothers query failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return nil, fmt.Errorf("list_conversations: brothers: %w", err)
	}
	out := make([]ConversationSummary, 0, len(brothers))
	for _, b := range brothers {
		conv, found, err := uc.conversations.FindDirectBetween(ctx, userID, b.ID)
		if err != nil {
			log.LogAttrs(ctx, slog.LevelError, "list_conversations: conversation lookup failed",
				logging.TargetUserID(b.ID),
				slog.String(logging.AttrError, err.Error()),
			)
			return nil, fmt.Errorf("list_conversations: lookup: %w", err)
		}

		var convID shared.ID
		if found {
			convID = conv.ID()
		}

		var avatarURL *string
		if b.AvatarKey != nil && uc.avatars != nil {
			u := uc.avatars.PublicURL(*b.AvatarKey)
			avatarURL = &u
		}
		out = append(out, ConversationSummary{
			ConversationID:   convID,
			BrotherID:        b.ID,
			BrotherUsername:  b.Username,
			BrotherStatus:    b.Status,
			BrotherAvatarURL: avatarURL,
			BecameBrothersAt: b.BecameBrothersAt,
			// LastMessageAt / Preview / UnreadCount are deferred to a
			// follow-up — the conversation list ships with brother
			// metadata only in the first iteration.
		})
	}
	log.LogAttrs(ctx, slog.LevelDebug, "conversations listed", slog.Int("count", len(out)))
	return out, nil
}
