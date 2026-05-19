package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

// PresignRateLimit is the in-process token bucket guarding the
// presign endpoint. Documented limit: 20 req/min, burst 5. Global
// (not per-user) on purpose — see "Known gaps" in the architecture
// doc.
//
// Rejected requests log at info level: a brief spike is normal, but
// sustained rejection means either an attack or a client bug.
func PresignRateLimit() func(http.Handler) http.Handler {
	limiter := rate.NewLimiter(rate.Every(3*time.Second), 5)
	return RateLimit(limiter, "presign")
}

// RateLimit is the underlying constructor exposed so tests can inject
// a controllable limiter.
func RateLimit(limiter *rate.Limiter, name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				logging.FromContext(r.Context()).LogAttrs(r.Context(), slog.LevelInfo, "rate limited",
					slog.String("limiter", name),
				)
				// Retry-After must be set before writeError flushes the
				// header block.
				w.Header().Set("Retry-After", "3")
				writeError(w, r, http.StatusTooManyRequests,
					"rate_limited", "Too many requests.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
