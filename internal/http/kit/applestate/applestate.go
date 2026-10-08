package applestate

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

const CookieName = "authara_apple_oauth"

const flowTTL = 10 * time.Minute

const maxPendingFlows = 8

var secureCookies = true

type Flow struct {
	State string
	Nonce string
}

func Configure(secure bool) {
	secureCookies = secure
}

func Create(w http.ResponseWriter, r *http.Request) (Flow, error) {
	state, err := randomValue()
	if err != nil {
		return Flow{}, err
	}
	nonce, err := randomValue()
	if err != nil {
		return Flow{}, err
	}
	flow := Flow{State: state, Nonce: nonce}
	flows := readAll(r)
	if len(flows) >= maxPendingFlows {
		flows = flows[len(flows)-maxPendingFlows+1:]
	}
	flows = append(flows, flow)
	write(w, flows)
	return flow, nil
}

// Consume returns and removes only the flow matching state. Other pending
// Apple flows remain usable, so opening sign-in in another tab does not
// invalidate the first tab's state and nonce.
func Consume(w http.ResponseWriter, r *http.Request, state string) (Flow, bool) {
	if state == "" {
		return Flow{}, false
	}
	flows := readAll(r)
	for i, flow := range flows {
		if subtle.ConstantTimeCompare([]byte(flow.State), []byte(state)) != 1 {
			continue
		}
		remaining := append(flows[:i:i], flows[i+1:]...)
		if len(remaining) == 0 {
			Clear(w)
		} else {
			write(w, remaining)
		}
		return flow, true
	}
	return Flow{}, false
}

func write(w http.ResponseWriter, flows []Flow) {
	values := make([]string, 0, len(flows))
	for _, flow := range flows {
		values = append(values, flow.State+"."+flow.Nonce)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    strings.Join(values, "|"),
		Path:     "/auth",
		Secure:   secureCookies,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(flowTTL.Seconds()),
	})
}

func readAll(r *http.Request) []Flow {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return nil
	}
	parts := strings.Split(cookie.Value, "|")
	if len(parts) > maxPendingFlows {
		parts = parts[len(parts)-maxPendingFlows:]
	}
	flows := make([]Flow, 0, len(parts))
	for _, part := range parts {
		state, nonce, ok := strings.Cut(part, ".")
		if !ok || state == "" || nonce == "" || strings.Contains(nonce, ".") {
			continue
		}
		flows = append(flows, Flow{State: state, Nonce: nonce})
	}
	return flows
}

func Clear(w http.ResponseWriter) {
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

func randomValue() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
