package email

import "time"

type LoginAlertPayload struct {
	V          int       `json:"v"`
	IPAddress  string    `json:"ip_address"`
	LoggedInAt time.Time `json:"logged_in_at"`
	UserAgent  string    `json:"user_agent,omitempty"`
}
