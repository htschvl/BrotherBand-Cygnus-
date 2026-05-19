// Package respond holds the HTTP response helpers shared by every
// handler and by the parent `adapter/http` package. It is a sibling
// package so the router (which imports handler/) and the handlers
// (which call Error, JSON, WriteSession, etc.) can both use the same
// primitives without forming an import cycle.
//
// `Error` is the single canonical place where a domain error becomes
// an HTTP response. Each domain sentinel is mapped exactly once; the
// fallback is 500 with a structured "internal_error" code.
package respond

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

// ErrorBody is the wire shape produced by Error.
//
//   - Code:      stable machine-readable identifier ("user.username_taken")
//   - Message:   short human-readable summary
//   - RequestID: always present — clients should surface this in bug reports
//   - Details:   optional structured payload (e.g. validation field/reason)
type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// Error translates a domain error into the appropriate HTTP response.
//
// Behaviour:
//   - The classified status, code, and message are derived from the
//     error chain via classify.
//   - Validation errors carry a typed ValidationError; the field name
//     and reason populate Details.
//   - 5xx responses are logged at error level with the request ID and
//     the original error message; the client never receives raw error
//     internals.
//   - 4xx responses are logged at info level with the resolved status
//     and code — this makes it trivial to grep for client-side issues
//     in production.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		err = errors.New("unspecified error")
	}
	status, code, message := classify(err)

	body := ErrorBody{
		Code:      code,
		Message:   message,
		RequestID: middleware.RequestIDFromContext(r.Context()),
	}
	if ve, ok := shared.AsValidationError(err); ok {
		body.Details = map[string]any{
			"field":  ve.Field,
			"reason": ve.Reason,
		}
	} else if multi, ok := asValidationErrors(err); ok {
		fields := make([]map[string]string, 0, len(multi.Items))
		for _, item := range multi.Items {
			fields = append(fields, map[string]string{"field": item.Field, "reason": item.Reason})
		}
		body.Details = map[string]any{"fields": fields}
	}

	logger := logging.FromContext(r.Context())
	switch {
	case status >= http.StatusInternalServerError:
		logger.LogAttrs(r.Context(), slog.LevelError, "request failed",
			slog.String(logging.AttrError, err.Error()),
			slog.String(logging.AttrErrorCode, code),
			slog.Int(logging.AttrStatus, status),
		)
	case status >= http.StatusBadRequest:
		logger.LogAttrs(r.Context(), slog.LevelInfo, "request rejected",
			slog.String(logging.AttrError, err.Error()),
			slog.String(logging.AttrErrorCode, code),
			slog.Int(logging.AttrStatus, status),
		)
	}
	JSON(w, status, body)
}

// JSON serialises any value as a JSON response.
//
// An encoding failure is logged but not propagated — by the time the
// status line has been written the HTTP response is committed and
// any further error return would be discarded by the framework.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("respond: json encode failed",
			slog.String(logging.AttrError, err.Error()),
			slog.Int(logging.AttrStatus, status),
		)
	}
}

// NoContent writes a 204 with no body. Used for idempotent mutations
// that don't return a representation.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// classify is a deliberately exhaustive switch over every domain
// error sentinel. The order matters: aggregate-specific errors are
// matched before the shared categories so a more precise code is
// always preferred.
func classify(err error) (int, string, string) {
	switch {
	// ─── User aggregate ──────────────────────────────────────────────
	case errors.Is(err, user.ErrUsernameAlreadyTaken):
		return http.StatusConflict, "user.username_taken", err.Error()
	case errors.Is(err, user.ErrInvalidCredentials):
		return http.StatusUnauthorized, "user.invalid_credentials", "Invalid credentials."
	case errors.Is(err, user.ErrPasswordTooWeak):
		return http.StatusUnprocessableEntity, "user.password_too_weak", err.Error()
	case errors.Is(err, user.ErrInvalidUsername):
		return http.StatusUnprocessableEntity, "user.invalid_username", err.Error()
	case errors.Is(err, user.ErrInvalidBirthdate):
		return http.StatusUnprocessableEntity, "user.invalid_birthdate", err.Error()
	case errors.Is(err, user.ErrInvalidSecret):
		return http.StatusUnprocessableEntity, "user.invalid_secret", err.Error()
	case errors.Is(err, user.ErrInvalidStatus):
		return http.StatusUnprocessableEntity, "user.invalid_status", err.Error()
	case errors.Is(err, user.ErrInvalidFavorites):
		return http.StatusUnprocessableEntity, "user.invalid_favorites", err.Error()
	case errors.Is(err, user.ErrNotFound):
		return http.StatusNotFound, "user.not_found", err.Error()

	// ─── Brotherband aggregate ───────────────────────────────────────
	case errors.Is(err, brotherband.ErrSelfRequest):
		return http.StatusUnprocessableEntity, "brotherband.self_request", err.Error()
	case errors.Is(err, brotherband.ErrAlreadyBrothers):
		return http.StatusConflict, "brotherband.already_brothers", err.Error()
	case errors.Is(err, brotherband.ErrRequestExists):
		return http.StatusConflict, "brotherband.request_exists", err.Error()
	case errors.Is(err, brotherband.ErrRequestNotFound):
		return http.StatusNotFound, "brotherband.request_not_found", err.Error()
	case errors.Is(err, brotherband.ErrNotABrother):
		return http.StatusForbidden, "brotherband.not_a_brother", err.Error()
	case errors.Is(err, brotherband.ErrNotRecipient):
		return http.StatusForbidden, "brotherband.not_recipient", err.Error()

	// ─── Message aggregate ───────────────────────────────────────────
	case errors.Is(err, message.ErrInvalidBody):
		return http.StatusUnprocessableEntity, "message.invalid_body", err.Error()
	case errors.Is(err, message.ErrInvalidAttachment):
		return http.StatusUnprocessableEntity, "message.invalid_attachment", err.Error()
	case errors.Is(err, message.ErrInvalidCursor):
		return http.StatusUnprocessableEntity, "message.invalid_cursor", err.Error()
	case errors.Is(err, message.ErrNotParticipant):
		return http.StatusForbidden, "message.not_participant", err.Error()
	case errors.Is(err, message.ErrConversationNotFound):
		return http.StatusNotFound, "message.conversation_not_found", err.Error()
	case errors.Is(err, message.ErrNotFound):
		return http.StatusNotFound, "message.not_found", err.Error()

	// ─── Media ───────────────────────────────────────────────────────
	case errors.Is(err, media.ErrUnsupportedMediaType):
		return http.StatusUnsupportedMediaType, "media.unsupported_type", err.Error()
	case errors.Is(err, media.ErrPayloadTooLarge):
		return http.StatusRequestEntityTooLarge, "media.payload_too_large", err.Error()
	case errors.Is(err, media.ErrPromotionFailed):
		return http.StatusConflict, "media.promotion_failed", err.Error()

	// ─── Shared fallback categories ──────────────────────────────────
	case errors.Is(err, shared.ErrNotFound):
		return http.StatusNotFound, "not_found", "Resource not found."
	case errors.Is(err, shared.ErrConflict):
		return http.StatusConflict, "conflict", err.Error()
	case errors.Is(err, shared.ErrForbidden):
		return http.StatusForbidden, "forbidden", err.Error()
	case errors.Is(err, shared.ErrUnauthenticated):
		return http.StatusUnauthorized, "unauthenticated", "Authentication required."
	case errors.Is(err, shared.ErrInvalidInput),
		errors.Is(err, shared.ErrInvalidID):
		return http.StatusUnprocessableEntity, "invalid_input", err.Error()

	default:
		return http.StatusInternalServerError, "internal_error", "An unexpected error occurred."
	}
}

// asValidationErrors mirrors errors.As for the aggregate type.
func asValidationErrors(err error) (*shared.ValidationErrors, bool) {
	var ve *shared.ValidationErrors
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}
