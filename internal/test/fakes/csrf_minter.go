package fakes

import (
	"strconv"
	"sync/atomic"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

// CSRFMinter produces deterministic, monotonic tokens so tests can
// assert exact cookie values.
type CSRFMinter struct {
	counter uint64
	prefix  string
}

// NewCSRFMinter returns a deterministic, monotonic CSRF minter for tests.
func NewCSRFMinter(prefix string) *CSRFMinter {
	if prefix == "" {
		prefix = "csrf"
	}
	return &CSRFMinter{prefix: prefix}
}

var _ port.CSRFMinter = (*CSRFMinter)(nil)

func (m *CSRFMinter) Mint() (string, error) {
	n := atomic.AddUint64(&m.counter, 1)
	return m.prefix + "-" + strconv.FormatUint(n, 10), nil
}
