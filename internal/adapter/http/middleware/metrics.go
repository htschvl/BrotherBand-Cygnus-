package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/infrastructure/observability"
)

// Metrics records counter and duration observations for every
// request, labelled by `method`, `route` (chi route pattern, not the
// concrete URL — bounded cardinality), and `status`.
func Metrics(m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unknown"
			}
			m.HTTPRequests.WithLabelValues(r.Method, route, strconv.Itoa(ww.Status())).Inc()
			m.HTTPDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		})
	}
}
