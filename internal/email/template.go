package email

import (
	"errors"
	"strings"

	"github.com/authara-org/authara/internal/email/templates"
)

var ErrInvalidOrganizationName = errors.New("invalid organization name")

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

func BuildPasswordResetCodeMessage(code string) (Message, error) {
	htmlBody, err := RenderPasswordResetCodeHTML(code)
	if err != nil {
		return Message{}, err
	}

	msg := Message{
		Subject: "Your password reset code",
		Text:    templates.PasswordResetCodeText(code),
		HTML:    htmlBody,
	}
	return msg, nil
}

func BuildEmailChangeCodeMessage(code string) (Message, error) {
	htmlBody, err := RenderEmailChangeCodeHTML(code)
	if err != nil {
		return Message{}, err
	}

	msg := Message{
		Subject: "Verify your new email address",
		Text:    templates.EmailChangeCodeText(code),
		HTML:    htmlBody,
	}
	return msg, nil
}

type OrganizationInvitationPayload struct {
	OrganizationName string `json:"organization_name"`
	InviteURL        string `json:"invite_url"`
	Role             string `json:"role"`
	ExpiresAt        string `json:"expires_at"`
}

func BuildOrganizationInvitationMessage(payload OrganizationInvitationPayload) (Message, error) {
	orgName := strings.TrimSpace(payload.OrganizationName)
	if orgName == "" {
		return Message{}, ErrInvalidOrganizationName
	}

	htmlBody, err := RenderOrganizationInvitationHTML(orgName, payload.InviteURL, payload.Role, payload.ExpiresAt)
	if err != nil {
		return Message{}, err
	}

	return Message{
		Subject: "You're invited to " + orgName,
		Text:    templates.OrganizationInvitationText(orgName, payload.InviteURL, payload.Role, payload.ExpiresAt),
		HTML:    htmlBody,
	}, nil
}
