package session

import (
	"net/http"
	"time"
)

const (
	accessCookieName  = "authgate_access"
	refreshCookieName = "authgate_refresh"

	accessCookiePath  = "/"
	refreshCookiePath = "/auth"
)

func SetAccessToken(w http.ResponseWriter, token string, maxAgeSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessCookieName,
		Value:    token,
		Path:     accessCookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAgeSeconds,
		Expires:  time.Now().Add(time.Duration(maxAgeSeconds) * time.Second),
	})
}

func SetRefreshToken(w http.ResponseWriter, token string, maxAgeSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAgeSeconds,
		Expires:  time.Now().Add(time.Duration(maxAgeSeconds) * time.Second),
	})
}

func ClearSessionCookies(w http.ResponseWriter) {
	expired := time.Unix(0, 0)

	http.SetCookie(w, &http.Cookie{
		Name:     accessCookieName,
		Value:    "",
		Path:     accessCookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expired,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
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
