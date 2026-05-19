package media_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	domainmedia "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fakes"
	usecasemedia "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/media"
)

func capCtx(t *testing.T) (context.Context, *logging.Capture) {
	t.Helper()
	c := logging.NewCapture(slog.LevelDebug)
	return logging.WithLogger(context.Background(), c.Logger()), c
}

func TestRequestUpload_HappyPath_LogsInfo(t *testing.T) {
	t.Parallel()
	uc := usecasemedia.NewRequestUpload(fakes.NewImageStore())
	ctx, c := capCtx(t)

	out, err := uc.Execute(ctx, usecasemedia.RequestUploadInput{
		OwnerID:       shared.NewID(),
		ContentType:   "image/webp",
		ContentLength: 4096,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.UploadURL == "" || out.MediaKey == "" {
		t.Fatalf("presign output incomplete: %#v", out)
	}
	if _, ok := c.FindByMessage("upload url presigned"); !ok {
		t.Fatal("expected INFO 'upload url presigned'")
	}
}

func TestRequestUpload_Rejections(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		contentType string
		length      int64
		want        error
	}{
		{"unsupported_type", "image/gif", 100, domainmedia.ErrUnsupportedMediaType},
		{"zero_length", "image/png", 0, domainmedia.ErrPayloadTooLarge},
		{"too_large", "image/png", domainmedia.MaxUploadBytes + 1, domainmedia.ErrPayloadTooLarge},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := usecasemedia.NewRequestUpload(fakes.NewImageStore())
			_, err := uc.Execute(context.Background(), usecasemedia.RequestUploadInput{
				OwnerID:       shared.NewID(),
				ContentType:   tc.contentType,
				ContentLength: tc.length,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			// The rejection must also be a typed ValidationError so the
			// HTTP layer can surface the offending field.
			if _, ok := shared.AsValidationError(err); !ok {
				t.Fatalf("rejection must be a ValidationError, got %T", err)
			}
		})
	}
}

func TestRequestUpload_StoreFailure_WrappedAndLoggedError(t *testing.T) {
	t.Parallel()
	uc := usecasemedia.NewRequestUpload(fakes.FailingImageStore{})
	ctx, c := capCtx(t)
	_, err := uc.Execute(ctx, usecasemedia.RequestUploadInput{
		OwnerID:       shared.NewID(),
		ContentType:   "image/jpeg",
		ContentLength: 2048,
	})
	if !errors.Is(err, fakes.ErrInjected) {
		t.Fatalf("expected wrapped injected error, got %v", err)
	}
	if _, ok := c.FindByMessage("request_upload: presign failed"); !ok {
		t.Fatal("presign failure must log at ERROR")
	}
}
