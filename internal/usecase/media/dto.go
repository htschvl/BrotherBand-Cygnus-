// Package media holds the use cases that orchestrate direct-to-R2
// image uploads. The handlers route a request here, the use case
// validates the input and delegates the actual signing to the
// `media.ImageStore` port. The R2 implementation lives in
// `adapter/storage/r2`.
package media

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// RequestUploadInput is the input to RequestUpload.Execute.
type RequestUploadInput struct {
	OwnerID       shared.ID
	ContentType   string
	ContentLength int64
}

// RequestUploadOutput is returned to the client; the bytes go
// directly to R2 in the next step.
type RequestUploadOutput struct {
	UploadURL string
	MediaKey  string
	ExpiresAt time.Time
}
