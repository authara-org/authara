package email

import (
	"fmt"
	"time"
)

func BuildSignupCodeMessage(code string) Message {
	subject := "Your verification code"

	text := fmt.Sprintf(
		"Your verification code is: %s\n\nIf you did not request this code, you can ignore this email.",
		code,
	)

	html := fmt.Sprintf(
		`<p>Your verification code is: <strong>%s</strong></p><p>If you did not request this code, you can ignore this email.</p>`,
		code,
	)

	return Message{
		Subject: subject,
		Text:    text,
		HTML:    html,
	}
}

func BuildLoginAlertMessage(p LoginAlertPayload) Message {
	userAgent := p.UserAgent
	if userAgent == "" {
		userAgent = "unknown device"
	}

	timestamp := p.LoggedInAt.UTC().Format(time.RFC3339)

	subject := "New login detected"

	text := fmt.Sprintf(
		"A new login was detected.\n\nIP address: %s\nTime: %s\nDevice: %s\n\nIf this was not you, please secure your account immediately.",
		p.IPAddress,
		timestamp,
		userAgent,
	)

	html := fmt.Sprintf(
		`<p>A new login was detected.</p>
<p><strong>IP address:</strong> %s<br><strong>Time:</strong> %s<br><strong>Device:</strong> %s</p>
<p>If this was not you, please secure your account immediately.</p>`,
		p.IPAddress,
		timestamp,
		userAgent,
	)

	return Message{
		Subject: subject,
		Text:    text,
		HTML:    html,
	}
}
