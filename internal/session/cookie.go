package session

import (
	"net/http"
	"time"
)

const (
	accessCookieName  = "authgate_access"
	refreshCookieName = "authgate_refresh"

	cookiePath = "/"
)

func SetAccessToken(w http.ResponseWriter, token string, maxAgeSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessCookieName,
		Value:    token,
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAgeSeconds,
	})
}

func SetRefreshToken(w http.ResponseWriter, token string, maxAgeSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAgeSeconds,
	})
}

func ClearSessionCookies(w http.ResponseWriter) {
	expired := time.Unix(0, 0)

	http.SetCookie(w, &http.Cookie{
		Name:     accessCookieName,
		Value:    "",
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expired,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expired,
		MaxAge:   -1,
	})
}

func ReadAccessToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(accessCookieName)
	if err != nil {
		return "", false
	}
	return cookie.Value, true
}

func ReadRefreshToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		return "", false
	}
	return cookie.Value, true
}
