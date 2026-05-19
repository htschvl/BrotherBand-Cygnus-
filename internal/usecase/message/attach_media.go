package message

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

const componentAttachMedia = "usecase.message.attach"

// AttachMedia attaches a previously uploaded media object to an
// existing message. Authorisation: only the message's sender may
// attach media to it.
//
// The use case promotes the object out of the `pending/` prefix into
// `messages/{conversationId}/{messageId}/{rest}` so the lifecycle
// rule does not later sweep it.
type AttachMedia struct {
	messages message.MessageWriter
	store    media.ImageStore
	clock    port.Clock
}

// NewAttachMedia wires the use case across the message writer and
// the image store.
func NewAttachMedia(messages message.MessageWriter, store media.ImageStore, clock port.Clock) *AttachMedia {
	return &AttachMedia{messages: messages, store: store, clock: clock}
}

func (uc *AttachMedia) Execute(ctx context.Context, in AttachInput) (MessageView, error) {
	log := logging.FromContext(ctx).With(
		logging.Component(componentAttachMedia),
		logging.UserID(in.ActorID),
		slog.String(logging.AttrMessageID, in.MessageID.String()),
	)

	if err := ctx.Err(); err != nil {
		return MessageView{}, fmt.Errorf("attach: context cancelled: %w", err)
	}

	msg, err := uc.messages.FindByID(ctx, in.MessageID)
	if err != nil {
		level := slog.LevelError
		if errors.Is(err, message.ErrNotFound) {
			level = slog.LevelInfo
		}
		log.LogAttrs(ctx, level, "attach: message lookup failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return MessageView{}, err
	}
	if !msg.SenderID().Equals(in.ActorID) {
		log.LogAttrs(ctx, slog.LevelWarn, "attach rejected: caller is not the sender",
			logging.TargetUserID(msg.SenderID()),
		)
		return MessageView{}, message.ErrNotParticipant
	}

	expectedPrefix := "pending/" + in.ActorID.String() + "/"
	if !strings.HasPrefix(in.MediaKey, expectedPrefix) {
		log.LogAttrs(ctx, slog.LevelWarn, "attach rejected: media key not owned by caller",
			slog.String(logging.AttrMediaKey, in.MediaKey),
		)
		return MessageView{}, media.ErrPromotionFailed
	}

	tail := strings.TrimPrefix(in.MediaKey, expectedPrefix)
	finalKey := "messages/" + msg.ConversationID().String() + "/" + msg.ID().String() + "/" + tail

	if err := uc.store.PromoteFromPending(ctx, in.MediaKey, finalKey); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "attach: promotion failed",
			slog.String(logging.AttrMediaKey, in.MediaKey),
			slog.String(logging.AttrError, err.Error()),
		)
		return MessageView{}, fmt.Errorf("attach: promote: %w", err)
	}

	contentType := contentTypeFromKey(finalKey)
	att, err := message.NewAttachment(msg.ID(), finalKey, contentType, sizeBytesUnknown, uc.clock.Now())
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "attach: domain attachment constructor failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return MessageView{}, err
	}
	att, err = uc.messages.SaveAttachment(ctx, att)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "attach: save attachment failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return MessageView{}, fmt.Errorf("attach: save: %w", err)
	}

	log.LogAttrs(ctx, slog.LevelInfo, "attachment recorded",
		slog.String(logging.AttrMediaKey, finalKey),
		slog.String(logging.AttrContentType, contentType),
		slog.String(logging.AttrEvent, "message.attachment_added"),
	)
	return toMessageView(msg.WithAttachments(append(msg.Attachments(), att)), uc.store), nil
}

// sizeBytesUnknown is the placeholder size used when the upload size
// is not echoed back from the storage adapter. R2 enforces the
// content length at upload time; recording the *exact* byte count
// would require a HEAD request which the architecture deems
// unnecessary for the MVP.
const sizeBytesUnknown int64 = 1

// contentTypeFromKey is the cheap inversion of the
// (contentType → extension) map used by the presigner. It avoids a
// HEAD round-trip to R2 just to discover the MIME type the client
// asked us to sign for.
func contentTypeFromKey(key string) string {
	switch {
	case strings.HasSuffix(key, ".jpg"), strings.HasSuffix(key, ".jpeg"):
		return string(media.JPEG)
	case strings.HasSuffix(key, ".png"):
		return string(media.PNG)
	case strings.HasSuffix(key, ".webp"):
		return string(media.WebP)
	default:
		return "application/octet-stream"
	}
}
