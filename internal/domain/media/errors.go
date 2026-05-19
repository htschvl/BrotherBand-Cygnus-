package media

import (
	"fmt"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

var (
	ErrUnsupportedMediaType = fmt.Errorf("media: unsupported content type: %w", shared.ErrInvalidInput)
	ErrPayloadTooLarge      = fmt.Errorf("media: payload exceeds the 10 MiB limit: %w", shared.ErrInvalidInput)
	ErrPromotionFailed      = fmt.Errorf("media: failed to promote pending object: %w", shared.ErrConflict)
)
