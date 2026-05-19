// Package handler holds the per-resource HTTP handlers. Each
// handler is a thin struct with one Execute-style method per route;
// it converts request → use-case input, calls the use case, and
// converts use-case output → response DTO. Nothing else.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// MaxRequestBodyBytes caps every JSON request body the API will
// accept. The presigned-upload payload itself goes directly to R2,
// so 1 MiB on the API surface is generous — anything larger is a
// client bug or an attack.
const MaxRequestBodyBytes int64 = 1 << 20

// decodeJSON reads a JSON body into `dest`. The error is normalised
// to a typed ValidationError so the error map produces a 422 with
// helpful field-level detail, never a raw parser stack trace.
func decodeJSON(r *http.Request, dest any) error {
	if r.Body == nil {
		return shared.WrapValidation(shared.ErrInvalidInput, "body", "request body is required")
	}
	r.Body = http.MaxBytesReader(nil, r.Body, MaxRequestBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return classifyDecodeError(err)
	}
	// Reject trailing garbage — `{"foo":1} extra` would otherwise pass.
	if dec.More() {
		return shared.WrapValidation(shared.ErrInvalidInput, "body", "unexpected data after JSON object")
	}
	return nil
}

// classifyDecodeError translates the family of json.Decoder errors
// into a typed ValidationError so the field name reaches the client.
// Each branch is documented because the json package's error messages
// are notoriously cryptic.
func classifyDecodeError(err error) error {
	switch {
	case errors.Is(err, io.EOF):
		return shared.WrapValidation(shared.ErrInvalidInput, "body", "request body is empty")
	case errors.Is(err, io.ErrUnexpectedEOF):
		// Truncated JSON (`{"a":`) reports as unexpected-EOF, not a
		// *json.SyntaxError — handle it explicitly so the client gets
		// "malformed JSON" instead of a cryptic stdlib string.
		return shared.WrapValidation(shared.ErrInvalidInput, "body", "malformed JSON: unexpected end of input")
	}

	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return shared.WrapValidation(
			shared.ErrInvalidInput,
			"body",
			fmt.Sprintf("request body must be at most %d bytes", maxBytes.Limit),
		)
	}

	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return shared.WrapValidation(
			shared.ErrInvalidInput,
			"body",
			fmt.Sprintf("malformed JSON at byte offset %d", syntax.Offset),
		)
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = "body"
		}
		return shared.WrapValidation(
			shared.ErrInvalidInput,
			field,
			fmt.Sprintf("expected %s, got %s", typeErr.Type.String(), typeErr.Value),
		)
	}

	if strings.HasPrefix(err.Error(), "json: unknown field ") {
		field := strings.TrimSuffix(strings.TrimPrefix(err.Error(), `json: unknown field "`), `"`)
		return shared.WrapValidation(shared.ErrInvalidInput, field, "field is not allowed")
	}

	return shared.WrapValidation(shared.ErrInvalidInput, "body", err.Error())
}
