package email

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/authara-org/authara/internal/domain"
)

const (
	TemplateVariableCode             = "code"
	TemplateVariableOrganizationName = "organization_name"
	TemplateVariableInviteURL        = "invite_url"
	TemplateVariableInvitationCode   = "invitation_code"
	TemplateVariableRole             = "role"
	TemplateVariableExpiresAt        = "expires_at"
)

var (
	ErrUnknownTemplate         = errors.New("unknown email template")
	ErrMissingTemplateVariable = errors.New("missing required email template variable")
)

// TemplateData contains the values available while rendering an email.
type TemplateData map[string]string

// TemplateDefinition describes one Core-owned transactional email and its
// fixed operator-editable contract.
type TemplateDefinition struct {
	Key         domain.EmailTemplate
	DisplayName string
	Description string

	DefaultSubjectTemplate string
	DefaultTextTemplate    string
	DefaultHTMLTemplate    string

	AvailableVariables       []string
	RequiredSubjectVariables []string
	RequiredBodyVariables    []string
	SampleData               TemplateData
}

var templateCatalog = []TemplateDefinition{
	{
		Key:                    domain.EmailTemplateSignupCode,
		DisplayName:            "Signup verification",
		Description:            "Sent when a user verifies their email address during signup.",
		DefaultSubjectTemplate: "Your verification code",
		DefaultTextTemplate:    defaultSignupCodeText,
		DefaultHTMLTemplate:    defaultSignupCodeHTML,
		AvailableVariables: []string{
			TemplateVariableCode,
		},
		RequiredBodyVariables: []string{
			TemplateVariableCode,
		},
		SampleData: TemplateData{
			TemplateVariableCode: "123456",
		},
	},
	{
		Key:                    domain.EmailTemplatePasswordResetCode,
		DisplayName:            "Password reset",
		Description:            "Sent when a user requests a password reset code.",
		DefaultSubjectTemplate: "Your password reset code",
		DefaultTextTemplate:    defaultPasswordResetCodeText,
		DefaultHTMLTemplate:    defaultPasswordResetCodeHTML,
		AvailableVariables: []string{
			TemplateVariableCode,
		},
		RequiredBodyVariables: []string{
			TemplateVariableCode,
		},
		SampleData: TemplateData{
			TemplateVariableCode: "123456",
		},
	},
	{
		Key:                    domain.EmailTemplateEmailChangeCode,
		DisplayName:            "Email address change",
		Description:            "Sent when a user verifies a new email address.",
		DefaultSubjectTemplate: "Verify your new email address",
		DefaultTextTemplate:    defaultEmailChangeCodeText,
		DefaultHTMLTemplate:    defaultEmailChangeCodeHTML,
		AvailableVariables: []string{
			TemplateVariableCode,
		},
		RequiredBodyVariables: []string{
			TemplateVariableCode,
		},
		SampleData: TemplateData{
			TemplateVariableCode: "123456",
		},
	},
	{
		Key:                    domain.EmailTemplateOrganizationInvite,
		DisplayName:            "Organization invitation",
		Description:            "Sent when a user is invited to join an organization.",
		DefaultSubjectTemplate: "You're invited to {{organization_name}}",
		DefaultTextTemplate:    defaultOrganizationInvitationText,
		DefaultHTMLTemplate:    defaultOrganizationInvitationHTML,
		AvailableVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariableInviteURL,
			TemplateVariableInvitationCode,
			TemplateVariableRole,
			TemplateVariableExpiresAt,
		},
		RequiredSubjectVariables: []string{
			TemplateVariableOrganizationName,
		},
		RequiredBodyVariables: []string{
			TemplateVariableInviteURL,
			TemplateVariableInvitationCode,
		},
		SampleData: TemplateData{
			TemplateVariableOrganizationName: "Acme Inc.",
			TemplateVariableInviteURL:        "https://authara.example/auth/invitations/accept?token=example",
			TemplateVariableInvitationCode:   "invite-example-123",
			TemplateVariableRole:             "member",
			TemplateVariableExpiresAt:        "2026-06-24T12:00:00Z",
		},
	},
}

// TemplateCatalog returns all supported email definitions in display order.
func TemplateCatalog() []TemplateDefinition {
	out := make([]TemplateDefinition, len(templateCatalog))
	for i, definition := range templateCatalog {
		out[i] = cloneTemplateDefinition(definition)
	}
	return out
}

// LookupTemplate returns the definition for a Core-owned email template.
func LookupTemplate(key domain.EmailTemplate) (TemplateDefinition, error) {
	for _, definition := range templateCatalog {
		if definition.Key == key {
			return cloneTemplateDefinition(definition), nil
		}
	}
	return TemplateDefinition{}, fmt.Errorf("%w: %q", ErrUnknownTemplate, key)
}

// ValidateTemplate checks whether Core owns the supplied email template type.
func ValidateTemplate(key domain.EmailTemplate) error {
	_, err := LookupTemplate(key)
	return err
}

// RenderBuiltInTemplate renders the canonical Authara default for a template.
func RenderBuiltInTemplate(key domain.EmailTemplate, data TemplateData) (Message, error) {
	definition, err := LookupTemplate(key)
	if err != nil {
		return Message{}, err
	}
	return renderTemplateSources(
		definition,
		definition.DefaultSubjectTemplate,
		definition.DefaultTextTemplate,
		definition.DefaultHTMLTemplate,
		data,
	)
}

func validateTemplateData(definition TemplateDefinition, data TemplateData) error {
	for _, variable := range definition.AvailableVariables {
		if strings.TrimSpace(data[variable]) == "" {
			return fmt.Errorf(
				"%w: template %q requires %q",
				ErrMissingTemplateVariable,
				definition.Key,
				variable,
			)
		}
	}
	return nil
}

func cloneTemplateDefinition(definition TemplateDefinition) TemplateDefinition {
	definition.AvailableVariables = slices.Clone(definition.AvailableVariables)
	definition.RequiredSubjectVariables = slices.Clone(definition.RequiredSubjectVariables)
	definition.RequiredBodyVariables = slices.Clone(definition.RequiredBodyVariables)
	definition.SampleData = maps.Clone(definition.SampleData)
	return definition
}
