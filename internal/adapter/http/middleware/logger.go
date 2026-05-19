package middleware

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

// Logger binds a *slog.Logger to the request context, pre-tagged
// with the request_id and (once the route is matched) the route
// pattern. Every downstream layer that calls `logging.FromContext(ctx)`
// then gets a logger that carries these attributes automatically.
//
// Mount order: must come AFTER RequestID (so we have an ID to tag
// with) and BEFORE any handler so the use cases can rely on it.
func Logger(base *slog.Logger) func(http.Handler) http.Handler {
	if base == nil {
		base = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := RequestIDFromContext(r.Context())
			logger := base.With(
				slog.String(logging.AttrRequestID, id),
				slog.String(logging.AttrMethod, r.Method),
				slog.String("path", r.URL.Path),
			)
			// Chi has not yet matched the route at the middleware
			// boundary; the access-log middleware is what records the
			// final route. We still attach a best-effort placeholder so
			// the handler can update it later via the Logger context.
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				logger = logger.With(slog.String(logging.AttrRoute, rc.RoutePattern()))
			}
			ctx := logging.WithLogger(r.Context(), logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
