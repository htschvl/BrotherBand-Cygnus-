package respond

import (
	"net/http"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/user"
)

// CookieConfig is the snapshot of cookie attributes that handlers
// need at write time.
type CookieConfig struct {
	Domain string
	Secure bool
}

// WriteSession writes the session and CSRF cookies in one place so
// register and login set them identically.
func WriteSession(w http.ResponseWriter, cfg CookieConfig, session user.Session) {
	maxAge := int(time.Until(session.Token.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CookieSession,
		Value:    session.Token.Value,
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   maxAge,
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CookieCSRF,
		Value:    session.CSRFToken,
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   maxAge,
		Secure:   cfg.Secure,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSession overwrites both cookies with empty values and
// MaxAge=-1 so the browser drops them.
func ClearSession(w http.ResponseWriter, cfg CookieConfig) {
	for _, name := range []string{middleware.CookieSession, middleware.CookieCSRF} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   cfg.Domain,
			MaxAge:   -1,
			Secure:   cfg.Secure,
			HttpOnly: name == middleware.CookieSession,
			SameSite: http.SameSiteLaxMode,
		})
	}
}
