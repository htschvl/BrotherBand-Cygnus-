package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

// HealthHandler exposes /healthz (cheap liveness) and /readyz
// (DB-backed readiness). Both responses are tiny so they are
// inexpensive to scrape.
type HealthHandler struct {
	pool    *pgxpool.Pool
	version string
}

// NewHealthHandler wires the liveness/readiness probes; pool may be nil only in tests that never call /readyz.
func NewHealthHandler(pool *pgxpool.Pool, version string) *HealthHandler {
	return &HealthHandler{pool: pool, version: version}
}

type healthBody struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

// Liveness always returns 200 if the process is up.
func (h *HealthHandler) Liveness(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, healthBody{Status: "ok", Version: h.version})
}

// Readiness returns 503 if Postgres is not reachable. Failures are
// logged at warn level so an outage is visible without spamming
// 5xx-level error logs every time a probe fires during a brief blip.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		logging.FromContext(r.Context()).LogAttrs(r.Context(), slog.LevelWarn, "readiness probe failed",
			slog.String(logging.AttrError, err.Error()),
		)
		respond.JSON(w, http.StatusServiceUnavailable, healthBody{Status: "degraded", Version: h.version})
		return
	}
	respond.JSON(w, http.StatusOK, healthBody{Status: "ok", Version: h.version})
}
