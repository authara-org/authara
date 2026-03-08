package session

import (
	"net/http"
	"time"
)

const (
	accessCookieName  = "authara_access"
	refreshCookieName = "authara_refresh"

	cookiePath = "/"
)

func SetAccessToken(w http.ResponseWriter, token string, maxAgeSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessCookieName,
		Value:    token,
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAgeSeconds,
		Expires:  time.Now().Add(time.Duration(maxAgeSeconds) * time.Second),
	})
}

func SetRefreshToken(w http.ResponseWriter, token string, maxAgeSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   secureCookies,
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
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   secureCookies,
		SameSite: http.SameSiteLaxMode,
		Expires:  expired,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   secureCookies,
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
