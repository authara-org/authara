package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/alexlup06-authgate/authgate/internal/http/csrf"
)

func CSRFOnGetMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only protect state-changing methods
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			next.ServeHTTP(w, r)
			return
		}

		if r.URL.Path == "/auth/oauth/google/callback" {
			next.ServeHTTP(w, r)
			return
		}

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
