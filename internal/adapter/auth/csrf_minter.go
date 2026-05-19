package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

// RandomCSRFMinter implements port.CSRFMinter by reading 32 bytes
// from crypto/rand and encoding them as URL-safe base64. The output
// is stored in the readable `bb_csrf` cookie and must match the
// `X-CSRF-Token` header on state-changing requests.
type RandomCSRFMinter struct{}

// NewRandomCSRFMinter returns the production CSRF minter (32 bytes from crypto/rand).
func NewRandomCSRFMinter() *RandomCSRFMinter { return &RandomCSRFMinter{} }

var _ port.CSRFMinter = (*RandomCSRFMinter)(nil)

func (RandomCSRFMinter) Mint() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("csrf: read random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
