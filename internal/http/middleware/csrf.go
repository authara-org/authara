package middleware

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/csrf"
	"github.com/authara-org/authara/internal/http/kit/response"
)

var (
	ErrCSRFCookieMissing = errors.New("csrf cookie token missing")
	ErrCSRFTokenMissing  = errors.New("csrf token missing in request")
	ErrCSRFTokenInvalid  = errors.New("csrf token invalid")
)

func validateCSRF(r *http.Request) error {
	c, err := r.Cookie(csrf.CookieName)
	if err != nil || c.Value == "" {
		return ErrCSRFCookieMissing
	}

	reqTok := r.Header.Get("X-CSRF-Token")
	if reqTok == "" {
		_ = r.ParseForm()
		reqTok = r.FormValue("csrf_token")
	}

	if reqTok == "" {
		return ErrCSRFTokenMissing
	}

	if subtle.ConstantTimeCompare([]byte(reqTok), []byte(c.Value)) != 1 {
		return ErrCSRFTokenInvalid
	}

	return nil
}

func RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := validateCSRF(r); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RequireAPICSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := validateCSRF(r); err != nil {
			response.ErrorJSON(
				w,
				http.StatusForbidden,
				response.CodeForbidden,
				"CSRF validation failed",
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}
