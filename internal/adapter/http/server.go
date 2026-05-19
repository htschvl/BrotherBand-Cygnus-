package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Server wraps net/http.Server with timeouts the architecture
// document specifies. Construction is via NewServer; lifecycle is
// driven by the composition root.
type Server struct {
	srv    *http.Server
	logger *slog.Logger
}

// ServerConfig is the small set of knobs the composition root sets.
type ServerConfig struct {
	Addr    string
	Handler http.Handler
	Logger  *slog.Logger
}

// NewServer builds a Server with safe defaults.
func NewServer(cfg ServerConfig) *Server {
	return &Server{
		srv: &http.Server{
			Addr:              cfg.Addr,
			Handler:           cfg.Handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
		logger: cfg.Logger,
	}
}

// ListenAndServe blocks until the server stops. It returns nil for
// the normal shutdown path and the underlying error otherwise.
func (s *Server) ListenAndServe() error {
	s.logger.Info("http server listening", slog.String("addr", s.srv.Addr))
	if err := s.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown drains in-flight requests up to `timeout` and then
// returns. The composition root calls this on SIGTERM/SIGINT.
func (s *Server) Shutdown(ctx context.Context, timeout time.Duration) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.srv.Shutdown(shutdownCtx)
}
