package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/handler"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/infrastructure/observability"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

// Routes groups every per-resource handler the router needs to mount.
type Routes struct {
	Auth        *handler.AuthHandler
	User        *handler.UserHandler
	Brotherband *handler.BrotherbandHandler
	Message     *handler.MessageHandler
	Media       *handler.MediaHandler
	Health      *handler.HealthHandler
}

// RouterConfig wires the cross-cutting concerns the middleware
// needs.
type RouterConfig struct {
	Logger         *slog.Logger
	Metrics        *observability.Metrics
	AllowedOrigins []string
	TokenIssuer    port.TokenIssuer
	Clock          port.Clock
}

// NewRouter assembles the chi router. It is the only place in the
// codebase that knows the path layout. Refactoring routes here
// rebuilds nothing else.
//
// Middleware order is intentional and load-bearing:
//
//  1. Recover     — top-of-stack panic safety net.
//  2. RequestID   — stable correlation key for every later layer.
//  3. Logger      — binds a request-id-tagged *slog.Logger to ctx.
//  4. AccessLog   — records the final status using the bound logger.
//  5. Metrics     — Prometheus counters/histograms.
//  6. CORS        — emits the right headers; handles OPTIONS preflight.
//  7. CSRF / Auth — gate state-changing / authenticated routes.
func NewRouter(cfg RouterConfig, routes Routes) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recover)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(cfg.Logger))
	r.Use(middleware.AccessLog(cfg.Logger))
	r.Use(middleware.Metrics(cfg.Metrics))
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	r.Get("/healthz", routes.Health.Liveness)
	r.Get("/readyz", routes.Health.Readiness)
	r.Method(http.MethodGet, "/metrics", promhttp.HandlerFor(cfg.Metrics.Registry, promhttp.HandlerOpts{}))

	r.Route("/v1", func(r chi.Router) {
		// Auth endpoints are deliberately NOT behind the CSRF
		// double-submit check. The check requires the client to echo
		// the bb_csrf cookie, but register/login are the endpoints
		// that *issue* that cookie — gating them on it is an
		// impossible bootstrap. Cross-site login/register POSTs are
		// already blocked by the SameSite=Lax session cookie and the
		// strict CORS allow-list; the double-submit token then guards
		// every *authenticated* state-changing request below. Logout
		// is safe to leave open: forcing a logout is a low-severity
		// annoyance, not a data-integrity risk, and SameSite=Lax
		// blocks the cross-site case anyway.
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", routes.Auth.Register)
			r.Post("/login", routes.Auth.Login)
			r.Post("/logout", routes.Auth.Logout)
		})

		// All other /v1 endpoints require an authenticated session.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(cfg.TokenIssuer, cfg.Clock))
			r.Use(middleware.CSRF)

			// Profile
			r.Get("/me", routes.User.GetMe)
			r.Patch("/me/status", routes.User.UpdateStatus)
			r.Patch("/me/avatar", routes.User.UpdateAvatar)

			// Brotherband
			r.Get("/brothers", routes.Brotherband.ListBrothers)
			r.Get("/brothers/{brotherId}", routes.Brotherband.GetBrother)
			r.Delete("/brothers/{brotherId}", routes.Brotherband.Cut)

			r.Get("/brotherband-requests", routes.Brotherband.ListRequests)
			r.Post("/brotherband-requests/send/{recipientId}", routes.Brotherband.Send)
			r.Post("/brotherband-requests/{requestId}/accept", routes.Brotherband.Accept)
			r.Post("/brotherband-requests/{requestId}/deny", routes.Brotherband.Deny)

			// Messages
			r.Get("/conversations", routes.Message.ListConversations)
			r.Get("/conversations/with/{brotherId}/messages", routes.Message.ListMessages)
			r.Post("/conversations/with/{brotherId}/messages", routes.Message.Send)
			r.Patch("/messages/{messageId}/attachment", routes.Message.Attach)

			// Media — gated by the in-process token bucket.
			r.With(middleware.PresignRateLimit()).
				Post("/media/upload-url", routes.Media.RequestUploadURL)
		})
	})

	return r
}
