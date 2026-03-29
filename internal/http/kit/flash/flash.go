package flash

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

const cookieName = "authara_flash"

type Message struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func Set(w http.ResponseWriter, msg Message) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    base64.RawURLEncoding.EncodeToString(b),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60,
	})
	return nil
}

func Read(w http.ResponseWriter, r *http.Request) (*Message, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil, nil
	}

	// clear immediately
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil, nil
	}

	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, nil
	}

	return &msg, nil
}
