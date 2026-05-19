package media

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

const componentRequestUpload = "usecase.media.request_upload"

// RequestUpload validates the requested content type and size, then
// asks the image store to mint a presigned PUT URL. The Go server's
// only cost per upload is one CPU-bound signing op + a Postgres
// write at attach time.
type RequestUpload struct {
	store media.ImageStore
}

// NewRequestUpload wires the use case against the image store port.
func NewRequestUpload(store media.ImageStore) *RequestUpload {
	return &RequestUpload{store: store}
}

func (uc *RequestUpload) Execute(ctx context.Context, in RequestUploadInput) (RequestUploadOutput, error) {
	log := logging.FromContext(ctx).With(
		logging.Component(componentRequestUpload),
		logging.UserID(in.OwnerID),
	)

	if err := ctx.Err(); err != nil {
		return RequestUploadOutput{}, fmt.Errorf("request_upload: context cancelled: %w", err)
	}

	contentType, err := media.ParseContentType(in.ContentType)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "request_upload rejected: bad content type",
			slog.String(logging.AttrContentType, in.ContentType),
			slog.String(logging.AttrError, err.Error()),
		)
		return RequestUploadOutput{}, err
	}
	if err := media.ValidateContentLength(in.ContentLength); err != nil {
		log.LogAttrs(ctx, slog.LevelDebug, "request_upload rejected: bad content length",
			slog.Int64(logging.AttrSizeBytes, in.ContentLength),
			slog.String(logging.AttrError, err.Error()),
		)
		return RequestUploadOutput{}, err
	}
	signed, err := uc.store.PresignUpload(ctx, in.OwnerID, contentType, in.ContentLength)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "request_upload: presign failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return RequestUploadOutput{}, fmt.Errorf("request_upload: presign: %w", err)
	}
	log.LogAttrs(ctx, slog.LevelInfo, "upload url presigned",
		slog.String(logging.AttrContentType, string(contentType)),
		slog.Int64(logging.AttrSizeBytes, in.ContentLength),
		slog.String(logging.AttrMediaKey, signed.MediaKey),
		slog.String(logging.AttrEvent, "media.presigned"),
	)
	return RequestUploadOutput{
		UploadURL: signed.UploadURL,
		MediaKey:  signed.MediaKey,
		ExpiresAt: signed.ExpiresAt,
	}, nil
}
