// Package clock implements port.Clock so business code never reaches
// for `time.Now()` directly.
//
// SystemClock is the production binding; tests inject Fixed{at} from
// their own helpers to make time-sensitive behaviour deterministic.
package clock

import (
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

// System is the production Clock — UTC, monotonic, real time.
type System struct{}

// New returns the production clock as a port.Clock for callers who
// want to wire by interface in their composition root.
func New() port.Clock { return System{} }

// Now returns the current UTC time. UTC is enforced so timestamps
// stored in the database are unambiguous regardless of host timezone.
func (System) Now() time.Time { return time.Now().UTC() }

// Fixed returns the same instant on every Now() call — used by tests.
type Fixed struct{ At time.Time }

func (f Fixed) Now() time.Time { return f.At }
