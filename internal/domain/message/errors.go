package message

import (
	"fmt"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

var (
	ErrInvalidBody          = fmt.Errorf("message: invalid body: %w", shared.ErrInvalidInput)
	ErrInvalidAttachment    = fmt.Errorf("message: invalid attachment: %w", shared.ErrInvalidInput)
	ErrInvalidCursor        = fmt.Errorf("message: invalid cursor: %w", shared.ErrInvalidInput)
	ErrNotParticipant       = fmt.Errorf("message: not a conversation participant: %w", shared.ErrForbidden)
	ErrNotFound             = fmt.Errorf("message: not found: %w", shared.ErrNotFound)
	ErrConversationNotFound = fmt.Errorf("message: conversation not found: %w", shared.ErrNotFound)
)
