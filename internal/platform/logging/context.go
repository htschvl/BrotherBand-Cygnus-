// Package logging provides the canonical seams every layer uses to
// emit structured log lines. The key idea: a `*slog.Logger` is
// attached to the request context by HTTP middleware so use cases,
// repositories, and adapters can retrieve a logger that already
// carries the request-correlation attributes — request ID, user ID,
// route — without each layer having to pass a logger argument
// through every function signature.
//
// `FromContext` is safe to call from any layer; it falls back to
// `slog.Default()` if no logger has been attached (so unit tests
// that build a use case without a context still produce log output
// when run with `-v` and configured stdlib slog).
package logging

import (
	"context"
	"log/slog"
)

type loggerKey struct{}

// WithLogger returns a child context with `logger` attached.
// Subsequent calls to FromContext within that subtree return this
// logger.
func WithLogger(parent context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return parent
	}
	return context.WithValue(parent, loggerKey{}, logger)
}

// FromContext returns the logger previously attached with WithLogger,
// or slog.Default() if none. The fallback is deliberate so any layer
// can call FromContext without having to check ok.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if v, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && v != nil {
		return v
	}
	return slog.Default()
}

// FromContextOr returns the logger attached with WithLogger, or the
// supplied fallback when none is bound (instead of slog.Default()).
// Middleware that accepts an injected base logger uses this so the
// base is a meaningful default even when the request context has not
// yet had a logger bound — e.g. when the middleware is mounted
// standalone in a test.
func FromContextOr(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if ctx != nil {
		if v, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && v != nil {
			return v
		}
	}
	if fallback != nil {
		return fallback
	}
	return slog.Default()
}

// With adds attributes to the context's logger and returns a new
// context carrying that enriched logger. The original parent is
// untouched.
func With(ctx context.Context, attrs ...any) context.Context {
	return WithLogger(ctx, FromContext(ctx).With(attrs...))
}
