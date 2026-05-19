package port

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// IssuedToken is the bundle returned by TokenIssuer.Issue. The HTTP
// adapter uses it to set the cookie and to choose the cookie's
// Max-Age attribute.
type IssuedToken struct {
	Value     string
	ExpiresAt time.Time
}

// TokenIssuer mints and validates the session token. The JWT adapter
// implements it; tests can substitute a deterministic fake.
type TokenIssuer interface {
	Issue(userID shared.ID, now time.Time) (IssuedToken, error)
	Verify(raw string, now time.Time) (shared.ID, error)
}
