package email

import (
	"errors"
	"strings"

	"github.com/authara-org/authara/internal/domain"
)

var ErrInvalidOrganizationName = errors.New("invalid organization name")

func BuildSignupCodeMessage(code string) (Message, error) {
	return RenderBuiltInTemplate(domain.EmailTemplateSignupCode, TemplateData{
		TemplateVariableCode: code,
	})
}

func BuildPasswordResetCodeMessage(code string) (Message, error) {
	return RenderBuiltInTemplate(domain.EmailTemplatePasswordResetCode, TemplateData{
		TemplateVariableCode: code,
	})
}

func BuildEmailChangeCodeMessage(code string) (Message, error) {
	return RenderBuiltInTemplate(domain.EmailTemplateEmailChangeCode, TemplateData{
		TemplateVariableCode: code,
	})
}

type OrganizationInvitationPayload struct {
	OrganizationName string `json:"organization_name"`
	InviteURL        string `json:"invite_url"`
	InvitationCode   string `json:"invitation_code,omitempty"`
	Role             string `json:"role"`
	ExpiresAt        string `json:"expires_at"`
}

func BuildOrganizationInvitationMessage(payload OrganizationInvitationPayload) (Message, error) {
	orgName := strings.TrimSpace(payload.OrganizationName)
	if orgName == "" {
		return Message{}, ErrInvalidOrganizationName
	}

	return RenderBuiltInTemplate(domain.EmailTemplateOrganizationInvite, TemplateData{
		TemplateVariableOrganizationName: orgName,
		TemplateVariableInviteURL:        payload.InviteURL,
		TemplateVariableInvitationCode:   payload.InvitationCode,
		TemplateVariableRole:             payload.Role,
		TemplateVariableExpiresAt:        payload.ExpiresAt,
	})
}
