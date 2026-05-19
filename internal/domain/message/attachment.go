package message

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// Attachment binds a message to an uploaded media object. The bytes
// live in R2; only the key + metadata travel through Postgres.
type Attachment struct {
	id          shared.ID
	messageID   shared.ID
	mediaKey    string
	contentType string
	sizeBytes   int64
	createdAt   time.Time
}

// Wire-name constants for attachment validation.
const (
	FieldMediaKey    = "mediaKey"
	FieldContentType = "contentType"
	FieldSizeBytes   = "sizeBytes"
)

// NewAttachment constructs a fresh attachment row. The use case
// validates that the caller is the message's sender; this constructor
// only checks structural invariants.
func NewAttachment(messageID shared.ID, mediaKey, contentType string, sizeBytes int64, now time.Time) (Attachment, error) {
	if mediaKey == "" {
		return Attachment{}, shared.WrapValidation(ErrInvalidAttachment, FieldMediaKey, "must not be empty")
	}
	if contentType == "" {
		return Attachment{}, shared.WrapValidation(ErrInvalidAttachment, FieldContentType, "must not be empty")
	}
	if sizeBytes <= 0 {
		return Attachment{}, shared.WrapValidation(ErrInvalidAttachment, FieldSizeBytes, "must be a positive integer")
	}
	return Attachment{
		id:          shared.NewID(),
		messageID:   messageID,
		mediaKey:    mediaKey,
		contentType: contentType,
		sizeBytes:   sizeBytes,
		createdAt:   now,
	}, nil
}

// RehydrateAttachment is the adapter-only constructor.
func RehydrateAttachment(id, messageID shared.ID, mediaKey, contentType string, sizeBytes int64, createdAt time.Time) Attachment {
	return Attachment{
		id: id, messageID: messageID, mediaKey: mediaKey,
		contentType: contentType, sizeBytes: sizeBytes, createdAt: createdAt,
	}
}

func (a Attachment) ID() shared.ID        { return a.id }
func (a Attachment) MessageID() shared.ID { return a.messageID }
func (a Attachment) MediaKey() string     { return a.mediaKey }
func (a Attachment) ContentType() string  { return a.contentType }
func (a Attachment) SizeBytes() int64     { return a.sizeBytes }
func (a Attachment) CreatedAt() time.Time { return a.createdAt }
