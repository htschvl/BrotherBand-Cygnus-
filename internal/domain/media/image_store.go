// Package media defines the port that the use cases use to obtain
// presigned upload URLs and to translate stored media keys into the
// public CDN URL clients can render.
//
// The implementation lives in `internal/adapter/storage/r2/` and is
// the only place that depends on the AWS SDK. Domain code only sees
// strings and a small set of typed errors.
package media

import (
	"context"
	"fmt"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// PresignedUpload is the value returned to the use case (and on to
// the client) when an upload is authorised.
type PresignedUpload struct {
	UploadURL string
	MediaKey  string
	ExpiresAt time.Time
}

// AllowedContentType is the closed set of MIME types the API will
// sign uploads for. Centralising it here means the HTTP DTO, the use
// case, and the storage adapter agree on exactly one list.
type AllowedContentType string

const (
	JPEG AllowedContentType = "image/jpeg"
	PNG  AllowedContentType = "image/png"
	WebP AllowedContentType = "image/webp"
)

// FieldContentType / FieldContentLength are the wire names exposed
// in field-level error responses.
const (
	FieldContentType   = "contentType"
	FieldContentLength = "contentLength"
)

// MaxUploadBytes is the hard cap on a single image upload (10 MiB).
// It is enforced both at presign time (signed Content-Length) and
// upstream by R2 itself.
const MaxUploadBytes int64 = 10 * 1024 * 1024

// AllowedTypes lists the closed set of permitted content types — the
// HTTP DTO uses this to render a stable error message and to drive
// the OpenAPI enum without duplication.
func AllowedTypes() []AllowedContentType {
	return []AllowedContentType{JPEG, PNG, WebP}
}

// ParseContentType validates a textual MIME type against the allow
// list and returns the typed value. The error is a typed
// ValidationError so the HTTP layer can render `details.field`.
func ParseContentType(raw string) (AllowedContentType, error) {
	switch AllowedContentType(raw) {
	case JPEG, PNG, WebP:
		return AllowedContentType(raw), nil
	default:
		return "", shared.WrapValidation(
			ErrUnsupportedMediaType,
			FieldContentType,
			fmt.Sprintf("must be one of %s, %s, %s", JPEG, PNG, WebP),
		)
	}
}

// ValidateContentLength returns nil if `n` falls within the
// (0, MaxUploadBytes] window, otherwise a typed ValidationError.
func ValidateContentLength(n int64) error {
	if n <= 0 {
		return shared.WrapValidation(ErrPayloadTooLarge, FieldContentLength, "must be a positive integer")
	}
	if n > MaxUploadBytes {
		return shared.WrapValidation(
			ErrPayloadTooLarge,
			FieldContentLength,
			fmt.Sprintf("must be at most %d bytes (10 MiB)", MaxUploadBytes),
		)
	}
	return nil
}

// ImageStore is the port the use cases depend on. The R2 presigner
// is one implementation; tests inject a fake.
type ImageStore interface {
	PresignUpload(ctx context.Context, ownerID shared.ID, contentType AllowedContentType, contentLength int64) (PresignedUpload, error)
	PublicURL(mediaKey string) string
	PromoteFromPending(ctx context.Context, pendingKey, finalKey string) error
}
