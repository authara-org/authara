package email

import (
	"github.com/authara-org/authara/internal/email/templates"
)

func BuildSignupCodeMessage(code string) (Message, error) {
	htmlBody, err := RenderSignupCodeHTML(code)
	if err != nil {
		return Message{}, err
	}

	msg := Message{
		Subject: "Your verification code",
		Text:    templates.SignupCodeText(code),
		HTML:    htmlBody,
	}
	return msg, nil
}
