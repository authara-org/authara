package oauthstate

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"
)

const CookieName = "authara_oauth_nonce"

const nonceTTL = 10 * time.Minute

var secureCookies = true

func Configure(secure bool) {
	secureCookies = secure
}

func Generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func EnsureNonce(w http.ResponseWriter, r *http.Request) (string, error) {
	c, err := r.Cookie(CookieName)
	if err == nil && c.Value != "" {
		return c.Value, nil
	}

	nonce, err := Generate()
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    nonce,
		Path:     "/auth",
		Secure:   secureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(nonceTTL.Seconds()),
	})

	return nonce, nil
}

func ReadNonce(r *http.Request) (string, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

func ClearNonce(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/auth",
		Secure:   secureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
