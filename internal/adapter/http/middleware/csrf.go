package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

// CookieCSRF and HeaderCSRF are the canonical names used in the
// double-submit pattern.
const (
	CookieCSRF = "bb_csrf"
	HeaderCSRF = "X-CSRF-Token"
)

// CSRF enforces the double-submit pattern on state-changing methods.
// GET / HEAD / OPTIONS are skipped because they are not allowed to
// have side effects.
//
// Rejections are logged at info level (they are a meaningful signal
// of either a misconfigured client or an attack) and produce a
// structured 403 body that includes the request ID.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(CookieCSRF)
		header := r.Header.Get(HeaderCSRF)
		switch {
		case err != nil:
			writeCSRFFailure(w, r, "missing_cookie")
			return
		case header == "":
			writeCSRFFailure(w, r, "missing_header")
			return
		case subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1:
			writeCSRFFailure(w, r, "token_mismatch")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func writeCSRFFailure(w http.ResponseWriter, r *http.Request, reason string) {
	logging.FromContext(r.Context()).LogAttrs(r.Context(), slog.LevelInfo, "csrf: rejected",
		slog.String("reason", reason),
	)
	writeError(w, r, http.StatusForbidden,
		"csrf.mismatch", "CSRF token missing or mismatched.")
}
