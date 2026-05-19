package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

// errorEnvelope is the JSON error shape middleware emits. It is a
// deliberate, minimal mirror of respond.ErrorBody. It is NOT imported
// from the respond package because respond imports this package for
// the cookie/CSRF constants — importing it back would create a cycle.
// A cross-package test (respond) asserts the byte-shape stays in sync.
type errorEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

// writeError is the single place every middleware writes an error
// body. Centralising it removes the previous hand-rolled JSON string
// literals (one per middleware), guarantees the envelope shape is
// defined exactly once on this side of the import boundary, and uses
// encoding/json so a request id containing a quote could never break
// the response.
//
// A failed Write is unrecoverable — the status line is already
// committed and the client has gone — so the error is intentionally
// discarded (idiomatic for the response-flush path).
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	body, err := json.Marshal(errorEnvelope{
		Code:      code,
		Message:   message,
		RequestID: RequestIDFromContext(r.Context()),
	})
	if err != nil {
		// Marshalling three plain strings cannot fail; this branch is
		// purely defensive so a future field change can never silently
		// turn into a write of nothing.
		logging.FromContext(r.Context()).Error("middleware: error envelope marshal failed",
			logging.Err(err))
		_, _ = w.Write([]byte(`{"code":"internal_error","message":"error encoding failed"}`))
		return
	}
	_, _ = w.Write(body)
}
