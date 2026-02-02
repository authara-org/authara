package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/alexlup06-authgate/authgate/internal/http/csrf"
)

func RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(csrf.CookieName)
		if err != nil || c.Value == "" {
			http.Error(w, "CSRF cookie token missing", http.StatusForbidden)
			return
		}

		reqTok := r.Header.Get("X-CSRF-Token")
		if reqTok == "" {
			_ = r.ParseForm()
			reqTok = r.FormValue("csrf_token")
		}

		if reqTok == "" {
			http.Error(w, "CSRF token missing in request", http.StatusForbidden)
			return
		}

		if subtle.ConstantTimeCompare([]byte(reqTok), []byte(c.Value)) != 1 {
			http.Error(w, "CSRF token invalid", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
