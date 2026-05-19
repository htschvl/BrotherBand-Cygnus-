package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

// CookieSession is the canonical name of the JWT-bearing cookie.
const CookieSession = "bb_session"

type userIDKey struct{}

// Auth verifies the bb_session cookie and threads the resolved user
// ID through the request context. Anonymous endpoints (login,
// register, health checks) must NOT mount this middleware.
//
// Side effects:
//   - On success the context-bound logger gets a `user_id` attribute
//     so every downstream log line is automatically tagged.
//   - On failure a debug-level log entry records the reason; the
//     response is 401 with a structured body that includes the
//     request ID.
func Auth(tokens port.TokenIssuer, clock port.Clock) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			logger := logging.FromContext(ctx)

			cookie, err := r.Cookie(CookieSession)
			if err != nil || cookie.Value == "" {
				logger.LogAttrs(ctx, slog.LevelDebug, "auth: missing session cookie",
					slog.String(logging.AttrError, errMessage(err)),
				)
				writeError(w, r, http.StatusUnauthorized,
					"unauthenticated", "Authentication required.")
				return
			}
			id, err := tokens.Verify(cookie.Value, clock.Now())
			if err != nil {
				logger.LogAttrs(ctx, slog.LevelDebug, "auth: invalid session token",
					slog.String(logging.AttrError, err.Error()),
				)
				writeError(w, r, http.StatusUnauthorized,
					"unauthenticated", "Authentication required.")
				return
			}
			ctx = context.WithValue(ctx, userIDKey{}, id)
			ctx = logging.WithLogger(ctx, logger.With(slog.String(logging.AttrUserID, id.String())))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext returns the authenticated user ID. Calling it
// from a route that does not mount Auth returns the zero ID.
func UserIDFromContext(ctx context.Context) shared.ID {
	if v, ok := ctx.Value(userIDKey{}).(shared.ID); ok {
		return v
	}
	return shared.ID{}
}

// errMessage renders an error for a log attribute, tolerating nil
// (the "no cookie at all" case has no underlying error).
func errMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
