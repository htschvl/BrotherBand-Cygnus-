package port

import "time"

// Clock is the seam that lets us write deterministic time-sensitive
// tests. Production wires `platform/clock.System{}`; tests wire a
// fixed-time fake. Domain code never calls `time.Now()` directly.
type Clock interface {
	Now() time.Time
}
