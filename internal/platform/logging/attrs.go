package logging

import (
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// Reusable attribute keys. Centralising them here means we can grep
// the codebase for log fields and prevents the inevitable drift of
// "user_id" vs "userID" vs "uid".
const (
	AttrRequestID    = "request_id"
	AttrUserID       = "user_id"
	AttrRoute        = "route"
	AttrMethod       = "method"
	AttrStatus       = "status"
	AttrDuration     = "duration"
	AttrBytes        = "bytes"
	AttrError        = "error"
	AttrErrorCode    = "error_code"
	AttrComponent    = "component"
	AttrUsername     = "username"
	AttrEvent        = "event"
	AttrTargetUserID = "target_user_id"
	AttrRequestUUID  = "brotherband_request_id"
	AttrMessageID    = "message_id"
	AttrConvID       = "conversation_id"
	AttrMediaKey     = "media_key"
	AttrContentType  = "content_type"
	AttrSizeBytes    = "size_bytes"
	AttrPanic        = "panic"
	AttrStack        = "stack"
)

// UserID is a convenience constructor that handles the zero-ID case
// without producing an attribute with an empty string value.
func UserID(id shared.ID) slog.Attr {
	if id.IsZero() {
		return slog.String(AttrUserID, "")
	}
	return slog.String(AttrUserID, id.String())
}

// TargetUserID is the same convenience for the "other" actor in a
// brotherband / messaging operation.
func TargetUserID(id shared.ID) slog.Attr {
	return slog.String(AttrTargetUserID, id.String())
}

// Component returns a `component=<name>` attribute. Use the package
// name; conventionally one component per package.
func Component(name string) slog.Attr {
	return slog.String(AttrComponent, name)
}

// Err wraps an error into a stable `error=<message>` attribute. The
// nil case is allowed because some helpers log both success and
// failure paths.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.Any(AttrError, nil)
	}
	return slog.String(AttrError, err.Error())
}
