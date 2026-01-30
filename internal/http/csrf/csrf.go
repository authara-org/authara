package csrf

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

const CookieName = "authgate_csrf"

func Generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func EnsureCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	c, err := r.Cookie(CookieName)
	if err == nil && c.Value != "" {
		return c.Value, nil
	}

	tok, err := Generate()
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    tok,
		Path:     "/",
		Secure:   true,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 30,
	})

	return tok, nil
}
