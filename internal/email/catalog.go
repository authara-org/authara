package email

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email/templates"
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

type templatePartRenderer func(TemplateData) (string, error)

// TemplateDefinition describes one Core-owned transactional email.
//
// SubjectTemplate is metadata for the future operator editor. Built-in email
// delivery continues to use the compiled renderers stored by the catalog.
type TemplateDefinition struct {
	Key               domain.EmailTemplate
	DisplayName       string
	Description       string
	SubjectTemplate   string
	RequiredVariables []string
	OptionalVariables []string
	SampleData        TemplateData
}

type templateCatalogEntry struct {
	definition    TemplateDefinition
	renderSubject templatePartRenderer
	renderText    templatePartRenderer
	renderHTML    templatePartRenderer
}

var templateCatalog = []templateCatalogEntry{
	{
		definition: TemplateDefinition{
			Key:             domain.EmailTemplateSignupCode,
			DisplayName:     "Signup verification",
			Description:     "Sent when a user verifies their email address during signup.",
			SubjectTemplate: "Your verification code",
			RequiredVariables: []string{
				TemplateVariableCode,
			},
			SampleData: TemplateData{
				TemplateVariableCode: "123456",
			},
		},
		renderSubject: staticTemplatePart("Your verification code"),
		renderText: func(data TemplateData) (string, error) {
			return templates.SignupCodeText(data[TemplateVariableCode]), nil
		},
		renderHTML: func(data TemplateData) (string, error) {
			return RenderSignupCodeHTML(data[TemplateVariableCode])
		},
	},
	{
		definition: TemplateDefinition{
			Key:             domain.EmailTemplatePasswordResetCode,
			DisplayName:     "Password reset",
			Description:     "Sent when a user requests a password reset code.",
			SubjectTemplate: "Your password reset code",
			RequiredVariables: []string{
				TemplateVariableCode,
			},
			SampleData: TemplateData{
				TemplateVariableCode: "123456",
			},
		},
		renderSubject: staticTemplatePart("Your password reset code"),
		renderText: func(data TemplateData) (string, error) {
			return templates.PasswordResetCodeText(data[TemplateVariableCode]), nil
		},
		renderHTML: func(data TemplateData) (string, error) {
			return RenderPasswordResetCodeHTML(data[TemplateVariableCode])
		},
	},
	{
		definition: TemplateDefinition{
			Key:             domain.EmailTemplateEmailChangeCode,
			DisplayName:     "Email address change",
			Description:     "Sent when a user verifies a new email address.",
			SubjectTemplate: "Verify your new email address",
			RequiredVariables: []string{
				TemplateVariableCode,
			},
			SampleData: TemplateData{
				TemplateVariableCode: "123456",
			},
		},
		renderSubject: staticTemplatePart("Verify your new email address"),
		renderText: func(data TemplateData) (string, error) {
			return templates.EmailChangeCodeText(data[TemplateVariableCode]), nil
		},
		renderHTML: func(data TemplateData) (string, error) {
			return RenderEmailChangeCodeHTML(data[TemplateVariableCode])
		},
	},
	{
		definition: TemplateDefinition{
			Key:             domain.EmailTemplateOrganizationInvite,
			DisplayName:     "Organization invitation",
			Description:     "Sent when a user is invited to join an organization.",
			SubjectTemplate: "You're invited to {{organization_name}}",
			RequiredVariables: []string{
				TemplateVariableOrganizationName,
				TemplateVariableInviteURL,
				TemplateVariableRole,
				TemplateVariableExpiresAt,
			},
			OptionalVariables: []string{
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
		renderSubject: func(data TemplateData) (string, error) {
			return "You're invited to " + strings.TrimSpace(data[TemplateVariableOrganizationName]), nil
		},
		renderText: func(data TemplateData) (string, error) {
			return templates.OrganizationInvitationText(
				strings.TrimSpace(data[TemplateVariableOrganizationName]),
				data[TemplateVariableInviteURL],
				data[TemplateVariableInvitationCode],
				data[TemplateVariableRole],
				data[TemplateVariableExpiresAt],
			), nil
		},
		renderHTML: func(data TemplateData) (string, error) {
			return RenderOrganizationInvitationHTML(
				strings.TrimSpace(data[TemplateVariableOrganizationName]),
				data[TemplateVariableInviteURL],
				data[TemplateVariableInvitationCode],
				data[TemplateVariableRole],
				data[TemplateVariableExpiresAt],
			)
		},
	},
}

// TemplateCatalog returns all supported email definitions in display order.
func TemplateCatalog() []TemplateDefinition {
	out := make([]TemplateDefinition, len(templateCatalog))
	for i, entry := range templateCatalog {
		out[i] = cloneTemplateDefinition(entry.definition)
	}
	return out
}

// LookupTemplate returns the definition for a Core-owned email template.
func LookupTemplate(key domain.EmailTemplate) (TemplateDefinition, error) {
	entry, err := lookupTemplateEntry(key)
	if err != nil {
		return TemplateDefinition{}, err
	}
	return cloneTemplateDefinition(entry.definition), nil
}

// ValidateTemplate checks whether Core owns the supplied email template type.
func ValidateTemplate(key domain.EmailTemplate) error {
	_, err := lookupTemplateEntry(key)
	return err
}

// RenderBuiltInTemplate renders the compiled Authara default for a template.
func RenderBuiltInTemplate(key domain.EmailTemplate, data TemplateData) (Message, error) {
	entry, err := lookupTemplateEntry(key)
	if err != nil {
		return Message{}, err
	}
	return entry.renderBuiltIn(data)
}

func (e templateCatalogEntry) renderBuiltIn(data TemplateData) (Message, error) {
	if err := validateTemplateData(e.definition, data); err != nil {
		return Message{}, err
	}

	subject, err := e.renderSubject(data)
	if err != nil {
		return Message{}, fmt.Errorf("render %q subject: %w", e.definition.Key, err)
	}
	textBody, err := e.renderText(data)
	if err != nil {
		return Message{}, fmt.Errorf("render %q text: %w", e.definition.Key, err)
	}
	htmlBody, err := e.renderHTML(data)
	if err != nil {
		return Message{}, fmt.Errorf("render %q HTML: %w", e.definition.Key, err)
	}

	return Message{
		Subject: subject,
		Text:    textBody,
		HTML:    htmlBody,
	}, nil
}

func validateTemplateData(definition TemplateDefinition, data TemplateData) error {
	for _, variable := range definition.RequiredVariables {
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

func lookupTemplateEntry(key domain.EmailTemplate) (templateCatalogEntry, error) {
	for _, entry := range templateCatalog {
		if entry.definition.Key == key {
			return entry, nil
		}
	}
	return templateCatalogEntry{}, fmt.Errorf("%w: %q", ErrUnknownTemplate, key)
}

func staticTemplatePart(value string) templatePartRenderer {
	return func(TemplateData) (string, error) {
		return value, nil
	}
}

func cloneTemplateDefinition(definition TemplateDefinition) TemplateDefinition {
	definition.RequiredVariables = slices.Clone(definition.RequiredVariables)
	definition.OptionalVariables = slices.Clone(definition.OptionalVariables)
	definition.SampleData = maps.Clone(definition.SampleData)
	return definition
}
