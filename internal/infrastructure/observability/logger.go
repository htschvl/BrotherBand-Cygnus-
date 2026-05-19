// Package observability bundles the slog handler factory and the
// Prometheus collector registration. Both are pure plumbing — the
// concerns they support (request IDs, access logging, metrics
// middleware) live in `adapter/http/middleware/`.
package observability

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger constructs the JSON slog logger and installs it as the
// process default so packages that reach for `slog.Default()` get
// the configured handler.
func NewLogger(levelText string) *slog.Logger {
	level := parseLevel(levelText)
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	l := slog.New(h)
	slog.SetDefault(l)
	return l
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
