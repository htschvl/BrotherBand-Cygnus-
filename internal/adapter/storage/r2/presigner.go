// Package r2 implements media.ImageStore against Cloudflare R2 via
// the AWS SDK v2. R2 is S3-compatible, so the SDK works unchanged
// with a custom endpoint and "auto" region.
//
// The adapter signs PUT URLs with a fixed Content-Length and
// Content-Type so R2 rejects oversized or mismatched uploads at the
// edge. Promotion (pending → final) is a server-side copy + delete.
package r2

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

// Presigner implements media.ImageStore.
type Presigner struct {
	client      *s3.Client
	bucket      string
	cdnBaseURL  string
	urlValidity time.Duration
}

// Config bundles the constructor inputs.
type Config struct {
	Client      *s3.Client
	Bucket      string
	CDNBaseURL  string
	URLValidity time.Duration
}

// NewPresigner constructs the adapter. URLValidity defaults to 15
// minutes when zero; matching the documented contract in the
// architecture doc.
func NewPresigner(cfg Config) *Presigner {
	if cfg.URLValidity == 0 {
		cfg.URLValidity = 15 * time.Minute
	}
	return &Presigner{
		client:      cfg.Client,
		bucket:      cfg.Bucket,
		cdnBaseURL:  cfg.CDNBaseURL,
		urlValidity: cfg.URLValidity,
	}
}

// Compile-time interface check.
var _ media.ImageStore = (*Presigner)(nil)

var contentTypeExtension = map[media.AllowedContentType]string{
	media.JPEG: ".jpg",
	media.PNG:  ".png",
	media.WebP: ".webp",
}

// PresignUpload returns a short-lived PUT URL bound to a specific
// content type and length. R2 rejects the upload if the headers
// echoed by the client diverge from what we signed.
func (p *Presigner) PresignUpload(
	ctx context.Context,
	ownerID shared.ID,
	contentType media.AllowedContentType,
	contentLength int64,
) (media.PresignedUpload, error) {
	ext, ok := contentTypeExtension[contentType]
	if !ok {
		return media.PresignedUpload{}, media.ErrUnsupportedMediaType
	}
	if contentLength <= 0 || contentLength > media.MaxUploadBytes {
		return media.PresignedUpload{}, media.ErrPayloadTooLarge
	}

	key := fmt.Sprintf("pending/%s/%s%s", ownerID.String(), uuid.NewString(), ext)
	expiresAt := time.Now().Add(p.urlValidity)

	presigner := s3.NewPresignClient(p.client)
	req, err := presigner.PresignPutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket:        aws.String(p.bucket),
			Key:           aws.String(key),
			ContentType:   aws.String(string(contentType)),
			ContentLength: aws.Int64(contentLength),
		},
		s3.WithPresignExpires(p.urlValidity),
	)
	if err != nil {
		return media.PresignedUpload{}, fmt.Errorf("r2: presign put: %w", err)
	}
	return media.PresignedUpload{
		UploadURL: req.URL,
		MediaKey:  key,
		ExpiresAt: expiresAt,
	}, nil
}

// PublicURL renders the CDN-fronted URL for a given key. Reads bypass
// the API server entirely.
func (p *Presigner) PublicURL(mediaKey string) string {
	return fmt.Sprintf("%s/%s", p.cdnBaseURL, mediaKey)
}

// PromoteFromPending moves a freshly uploaded object out of the
// `pending/` prefix so it isn't swept by the 24 h lifecycle rule.
// R2 has no atomic rename, so we copy then delete; in the rare
// failure mode where the copy succeeds but the delete fails the
// orphan in `pending/` is reaped automatically.
func (p *Presigner) PromoteFromPending(ctx context.Context, pendingKey, finalKey string) error {
	if pendingKey == "" || finalKey == "" {
		return media.ErrPromotionFailed
	}
	log := logging.FromContext(ctx).With(
		logging.Component("adapter.storage.r2"),
		slog.String("pending_key", pendingKey),
		slog.String("final_key", finalKey),
	)

	_, err := p.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(p.bucket),
		Key:        aws.String(finalKey),
		CopySource: aws.String(p.bucket + "/" + pendingKey),
	})
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "r2: copy failed during promotion",
			slog.String(logging.AttrError, err.Error()),
		)
		return fmt.Errorf("r2: copy: %w", errors.Join(media.ErrPromotionFailed, err))
	}
	if _, delErr := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(pendingKey),
	}); delErr != nil {
		// The copy succeeded, so the promotion is logically complete.
		// The leftover pending object is swept by the 24 h R2 lifecycle
		// rule; we log WARN (not ERROR) because it self-heals.
		log.LogAttrs(ctx, slog.LevelWarn, "r2: pending object delete failed; orphan will be lifecycle-swept",
			slog.String(logging.AttrError, delErr.Error()),
		)
	}
	return nil
}
