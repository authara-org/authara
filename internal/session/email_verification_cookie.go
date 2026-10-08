package session

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

const emailVerificationCookieName = "authara_email_verification"

func SetEmailVerificationTransaction(w http.ResponseWriter, id uuid.UUID, expiresAt time.Time, now time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     emailVerificationCookieName,
		Value:    id.String(),
		Path:     "/auth/verify-email",
		HttpOnly: true,
		Secure:   secureCookies,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   max(1, int(expiresAt.Sub(now).Seconds())),
	})
}

func ReadEmailVerificationTransaction(r *http.Request) (uuid.UUID, bool) {
	cookie, err := r.Cookie(emailVerificationCookieName)
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(cookie.Value)
	return id, err == nil && id != uuid.Nil
}

func ClearEmailVerificationTransaction(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     emailVerificationCookieName,
		Path:     "/auth/verify-email",
		HttpOnly: true,
		Secure:   secureCookies,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}
