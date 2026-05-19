package brotherband

import (
	"fmt"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

var (
	ErrSelfRequest     = fmt.Errorf("brotherband: cannot request yourself: %w", shared.ErrInvalidInput)
	ErrAlreadyBrothers = fmt.Errorf("brotherband: users are already brothers: %w", shared.ErrConflict)
	ErrRequestExists   = fmt.Errorf("brotherband: a request between these users already exists: %w", shared.ErrConflict)
	ErrRequestNotFound = fmt.Errorf("brotherband: request not found: %w", shared.ErrNotFound)
	ErrNotABrother     = fmt.Errorf("brotherband: users are not brothers: %w", shared.ErrForbidden)
	ErrNotRecipient    = fmt.Errorf("brotherband: only the recipient can act on this request: %w", shared.ErrForbidden)
)
