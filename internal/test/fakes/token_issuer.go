package fakes

import (
	"sync"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

// TokenIssuer is the in-memory port.TokenIssuer. The "token" is just
// the user ID prefixed with "tok-"; tests never inspect it for
// shape, only for round-trip equality.
type TokenIssuer struct {
	mu     sync.RWMutex
	issued map[string]shared.ID
	TTL    time.Duration
}

// NewTokenIssuer returns an in-memory token issuer for tests.
func NewTokenIssuer() *TokenIssuer {
	return &TokenIssuer{issued: map[string]shared.ID{}, TTL: 30 * 24 * time.Hour}
}

var _ port.TokenIssuer = (*TokenIssuer)(nil)

func (t *TokenIssuer) Issue(userID shared.ID, now time.Time) (port.IssuedToken, error) {
	tok := "tok-" + userID.String()
	t.mu.Lock()
	t.issued[tok] = userID
	t.mu.Unlock()
	return port.IssuedToken{Value: tok, ExpiresAt: now.Add(t.TTL)}, nil
}

func (t *TokenIssuer) Verify(raw string, _ time.Time) (shared.ID, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	id, ok := t.issued[raw]
	if !ok {
		return shared.ID{}, shared.ErrUnauthenticated
	}
	return id, nil
}
