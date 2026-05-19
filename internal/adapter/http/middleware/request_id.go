// Package middleware contains the chi-compatible
// `func(http.Handler) http.Handler` wrappers used by every route.
//
// The order they are mounted in matters; the canonical order applied
// in router.go is:
//
//	Recover → RequestID → Logger → AccessLog → Metrics → CORS → Auth (protected) → CSRF (authed, state-changing) → handler.
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// RequestIDHeader is the canonical HTTP header for request
// correlation. It is read on inbound requests and echoed in the
// response so a client can correlate retries.
const RequestIDHeader = "X-Request-ID"

type requestIDKey struct{}

// RequestID assigns each request a stable identifier and threads it
// through the context so the access log, the error map and any
// downstream slog calls can include it.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the ID stored by RequestID, or "".
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}
