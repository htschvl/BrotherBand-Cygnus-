package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

// AccessLog emits one structured line per request. The status code,
// bytes written, and elapsed time come from chi's WrapResponseWriter
// so we observe the actual response without buffering the body.
//
// Verbosity follows the response: 5xx is logged at error level, 4xx
// at warn level, success at info level.
func AccessLog(base *slog.Logger) func(http.Handler) http.Handler {
	if base == nil {
		base = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			level := slog.LevelInfo
			switch {
			case ww.Status() >= http.StatusInternalServerError:
				level = slog.LevelError
			case ww.Status() >= http.StatusBadRequest:
				level = slog.LevelWarn
			}

			logging.FromContextOr(r.Context(), base).LogAttrs(r.Context(), level, "http request",
				slog.String(logging.AttrRequestID, RequestIDFromContext(r.Context())),
				slog.String(logging.AttrMethod, r.Method),
				slog.String("path", r.URL.Path),
				slog.String(logging.AttrRoute, route),
				slog.Int(logging.AttrStatus, ww.Status()),
				slog.Int(logging.AttrBytes, ww.BytesWritten()),
				slog.Duration(logging.AttrDuration, time.Since(start)),
			)
		})
	}
}
