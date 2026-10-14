package oauthstate

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

const CookieName = "authara_oauth_nonce"

const nonceTTL = 10 * time.Minute

const maxPendingNonces = 8

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
	nonces := readAll(r)
	if len(nonces) > 0 {
		return nonces[len(nonces)-1], nil
	}
	return Create(w, r)
}

func Create(w http.ResponseWriter, r *http.Request) (string, error) {
	nonce, err := Generate()
	if err != nil {
		return "", err
	}
	nonces := readAll(r)
	if len(nonces) >= maxPendingNonces {
		for _, expired := range nonces[:len(nonces)-maxPendingNonces+1] {
			clearFlowCookie(w, expired)
		}
		nonces = nonces[len(nonces)-maxPendingNonces+1:]
	}
	write(w, append(nonces, nonce))
	setFlowCookie(w, nonce)
	return nonce, nil
}

// Consume removes only the matching nonce so other tabs and providers keep
// their pending OAuth flows.
func Consume(w http.ResponseWriter, r *http.Request, nonce string) (string, bool) {
	if nonce == "" {
		return "", false
	}
	nonces := readAll(r)
	for i, candidate := range nonces {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(nonce)) != 1 {
			continue
		}
		remaining := append(nonces[:i:i], nonces[i+1:]...)
		if len(remaining) == 0 {
			ClearNonce(w)
		} else {
			write(w, remaining)
		}
		clearFlowCookie(w, candidate)
		return candidate, true
	}
	flowCookie, err := r.Cookie(flowCookieName(nonce))
	if err == nil && subtle.ConstantTimeCompare([]byte(flowCookie.Value), []byte(nonce)) == 1 {
		clearFlowCookie(w, nonce)
		return nonce, true
	}
	return "", false
}

func write(w http.ResponseWriter, nonces []string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    strings.Join(nonces, "|"),
		Path:     "/auth",
		Secure:   secureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(nonceTTL.Seconds()),
	})
}

func setFlowCookie(w http.ResponseWriter, nonce string) {
	http.SetCookie(w, &http.Cookie{
		Name:     flowCookieName(nonce),
		Value:    nonce,
		Path:     "/auth",
		Secure:   secureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(nonceTTL.Seconds()),
	})
}

func clearFlowCookie(w http.ResponseWriter, nonce string) {
	http.SetCookie(w, &http.Cookie{
		Name:     flowCookieName(nonce),
		Value:    "",
		Path:     "/auth",
		Secure:   secureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func flowCookieName(nonce string) string {
	digest := sha256.Sum256([]byte(nonce))
	return CookieName + "_" + hex.EncodeToString(digest[:8])
}

func ReadNonce(r *http.Request) (string, bool) {
	nonces := readAll(r)
	if len(nonces) == 0 {
		return "", false
	}
	return nonces[len(nonces)-1], true
}

func readAll(r *http.Request) []string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return nil
	}
	parts := strings.Split(cookie.Value, "|")
	if len(parts) > maxPendingNonces {
		parts = parts[len(parts)-maxPendingNonces:]
	}
	nonces := make([]string, 0, len(parts))
	for _, nonce := range parts {
		if nonce == "" || strings.Contains(nonce, "|") {
			continue
		}
		nonces = append(nonces, nonce)
	}
	return nonces
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
