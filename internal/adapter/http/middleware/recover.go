package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

// Recover converts a panic in any downstream handler into a
// structured 500 response, recording the stack trace at error level
// alongside the request ID.
//
// This is the load-bearing safety net at the top of the middleware
// stack: nothing downstream can take the process down with a panic.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logging.FromContext(r.Context()).LogAttrs(r.Context(), slog.LevelError, "handler panic",
					slog.Any(logging.AttrPanic, rec),
					slog.String(logging.AttrStack, string(debug.Stack())),
				)
				writeError(w, r, http.StatusInternalServerError,
					"internal_error", "An unexpected error occurred.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
