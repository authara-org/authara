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
	TemplateVariableCode               = "code"
	TemplateVariableOrganizationName   = "organization_name"
	TemplateVariableInviteURL          = "invite_url"
	TemplateVariableInvitationCode     = "invitation_code"
	TemplateVariableRole               = "role"
	TemplateVariableExpiresAt          = "expires_at"
	TemplateVariableUsername           = "username"
	TemplateVariableAuthMethod         = "auth_method"
	TemplateVariableOccurredAt         = "occurred_at"
	TemplateVariableIPAddress          = "ip_address"
	TemplateVariableUserAgent          = "user_agent"
	TemplateVariableOldEmail           = "old_email"
	TemplateVariableNewEmail           = "new_email"
	TemplateVariableAccessChange       = "access_change"
	TemplateVariableMemberEmail        = "member_email"
	TemplateVariablePreviousRole       = "previous_role"
	TemplateVariablePreviousOwnerEmail = "previous_owner_email"
	TemplateVariableNewOwnerEmail      = "new_owner_email"
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
	{
		Key:                    domain.EmailTemplateAccountCreated,
		DisplayName:            "Account created",
		Description:            "Sent after a password or external-provider signup completes.",
		DefaultSubjectTemplate: "Welcome to Authara",
		DefaultTextTemplate:    defaultAccountCreated.Text,
		DefaultHTMLTemplate:    defaultAccountCreated.HTML,
		AvailableVariables: []string{
			TemplateVariableUsername,
			TemplateVariableAuthMethod,
			TemplateVariableOccurredAt,
		},
		RequiredBodyVariables: []string{
			TemplateVariableUsername,
			TemplateVariableAuthMethod,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableUsername:   "alex",
			TemplateVariableAuthMethod: "google",
			TemplateVariableOccurredAt: "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateNewSignIn,
		DisplayName:            "New sign-in",
		Description:            "Sent whenever a new authenticated session is created.",
		DefaultSubjectTemplate: "New sign-in to your Authara account",
		DefaultTextTemplate:    defaultNewSignIn.Text,
		DefaultHTMLTemplate:    defaultNewSignIn.HTML,
		AvailableVariables: []string{
			TemplateVariableIPAddress,
			TemplateVariableUserAgent,
			TemplateVariableOccurredAt,
		},
		RequiredBodyVariables: []string{
			TemplateVariableIPAddress,
			TemplateVariableUserAgent,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableIPAddress:  "203.0.113.42",
			TemplateVariableUserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			TemplateVariableOccurredAt: "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateAuthMethodAdded,
		DisplayName:            "Sign-in method added",
		Description:            "Sent after a password, external provider, or passkey is added to an existing account.",
		DefaultSubjectTemplate: "A sign-in method was added",
		DefaultTextTemplate:    defaultAuthMethodAdded.Text,
		DefaultHTMLTemplate:    defaultAuthMethodAdded.HTML,
		AvailableVariables: []string{
			TemplateVariableAuthMethod,
			TemplateVariableOccurredAt,
		},
		RequiredBodyVariables: []string{
			TemplateVariableAuthMethod,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableAuthMethod: "google",
			TemplateVariableOccurredAt: "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateAuthMethodRemoved,
		DisplayName:            "Sign-in method removed",
		Description:            "Sent after a password, external provider, or passkey is removed from an account.",
		DefaultSubjectTemplate: "A sign-in method was removed",
		DefaultTextTemplate:    defaultAuthMethodRemoved.Text,
		DefaultHTMLTemplate:    defaultAuthMethodRemoved.HTML,
		AvailableVariables: []string{
			TemplateVariableAuthMethod,
			TemplateVariableOccurredAt,
		},
		RequiredBodyVariables: []string{
			TemplateVariableAuthMethod,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableAuthMethod: "passkey (MacBook Touch ID)",
			TemplateVariableOccurredAt: "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplatePasswordChanged,
		DisplayName:            "Password changed",
		Description:            "Sent after an authenticated password change, password reset, or password replacement.",
		DefaultSubjectTemplate: "Your Authara password was changed",
		DefaultTextTemplate:    defaultPasswordChanged.Text,
		DefaultHTMLTemplate:    defaultPasswordChanged.HTML,
		AvailableVariables: []string{
			TemplateVariableOccurredAt,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOccurredAt: "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateEmailChangedOldAddress,
		DisplayName:            "Email changed — previous address",
		Description:            "Security notice sent to the previous address after an account email change.",
		DefaultSubjectTemplate: "Your Authara email address was changed",
		DefaultTextTemplate:    defaultEmailChangedOldAddress.Text,
		DefaultHTMLTemplate:    defaultEmailChangedOldAddress.HTML,
		AvailableVariables: []string{
			TemplateVariableOldEmail,
			TemplateVariableNewEmail,
			TemplateVariableOccurredAt,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOldEmail,
			TemplateVariableNewEmail,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOldEmail:   "alex.old@example.com",
			TemplateVariableNewEmail:   "alex.new@example.com",
			TemplateVariableOccurredAt: "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateEmailChangedNewAddress,
		DisplayName:            "Email changed — new address",
		Description:            "Confirmation sent to the new address after an account email change.",
		DefaultSubjectTemplate: "Your new Authara email address is active",
		DefaultTextTemplate:    defaultEmailChangedNewAddress.Text,
		DefaultHTMLTemplate:    defaultEmailChangedNewAddress.HTML,
		AvailableVariables: []string{
			TemplateVariableOldEmail,
			TemplateVariableNewEmail,
			TemplateVariableOccurredAt,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOldEmail,
			TemplateVariableNewEmail,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOldEmail:   "alex.old@example.com",
			TemplateVariableNewEmail:   "alex.new@example.com",
			TemplateVariableOccurredAt: "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateAccountDisabled,
		DisplayName:            "Account disabled",
		Description:            "Sent after an administrator disables an account and revokes its sessions.",
		DefaultSubjectTemplate: "Your Authara account was disabled",
		DefaultTextTemplate:    defaultAccountDisabled.Text,
		DefaultHTMLTemplate:    defaultAccountDisabled.HTML,
		AvailableVariables: []string{
			TemplateVariableOccurredAt,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOccurredAt: "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateAccountEnabled,
		DisplayName:            "Account enabled",
		Description:            "Sent after an administrator enables a disabled account.",
		DefaultSubjectTemplate: "Your Authara account was enabled",
		DefaultTextTemplate:    defaultAccountEnabled.Text,
		DefaultHTMLTemplate:    defaultAccountEnabled.HTML,
		AvailableVariables: []string{
			TemplateVariableOccurredAt,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOccurredAt: "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateAdminAccessChanged,
		DisplayName:            "Administrator access changed",
		Description:            "Sent after administrator access is granted or revoked.",
		DefaultSubjectTemplate: "Your Authara administrator access was {{access_change}}",
		DefaultTextTemplate:    defaultAdminAccessChanged.Text,
		DefaultHTMLTemplate:    defaultAdminAccessChanged.HTML,
		AvailableVariables: []string{
			TemplateVariableAccessChange,
			TemplateVariableOccurredAt,
		},
		RequiredSubjectVariables: []string{
			TemplateVariableAccessChange,
		},
		RequiredBodyVariables: []string{
			TemplateVariableAccessChange,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableAccessChange: "granted",
			TemplateVariableOccurredAt:   "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateOrganizationInvitationAccepted,
		DisplayName:            "Organization invitation accepted",
		Description:            "Sent to the inviter after a user accepts an organization invitation.",
		DefaultSubjectTemplate: "Invitation accepted for {{organization_name}}",
		DefaultTextTemplate:    defaultOrganizationInvitationAccepted.Text,
		DefaultHTMLTemplate:    defaultOrganizationInvitationAccepted.HTML,
		AvailableVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariableMemberEmail,
			TemplateVariableRole,
			TemplateVariableOccurredAt,
		},
		RequiredSubjectVariables: []string{
			TemplateVariableOrganizationName,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariableMemberEmail,
			TemplateVariableRole,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOrganizationName: "Acme Inc.",
			TemplateVariableMemberEmail:      "member@example.com",
			TemplateVariableRole:             "member",
			TemplateVariableOccurredAt:       "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateOrganizationInvitationRevoked,
		DisplayName:            "Organization invitation revoked",
		Description:            "Sent to the invitee when a pending organization invitation is explicitly revoked.",
		DefaultSubjectTemplate: "Your invitation to {{organization_name}} was revoked",
		DefaultTextTemplate:    defaultOrganizationInvitationRevoked.Text,
		DefaultHTMLTemplate:    defaultOrganizationInvitationRevoked.HTML,
		AvailableVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariableOccurredAt,
		},
		RequiredSubjectVariables: []string{
			TemplateVariableOrganizationName,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOrganizationName: "Acme Inc.",
			TemplateVariableOccurredAt:       "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateOrganizationMembershipRemoved,
		DisplayName:            "Organization membership removed",
		Description:            "Sent after a user leaves or is removed from an organization.",
		DefaultSubjectTemplate: "Your access to {{organization_name}} was removed",
		DefaultTextTemplate:    defaultOrganizationMembershipRemoved.Text,
		DefaultHTMLTemplate:    defaultOrganizationMembershipRemoved.HTML,
		AvailableVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariableRole,
			TemplateVariableOccurredAt,
		},
		RequiredSubjectVariables: []string{
			TemplateVariableOrganizationName,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariableRole,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOrganizationName: "Acme Inc.",
			TemplateVariableRole:             "member",
			TemplateVariableOccurredAt:       "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateOrganizationRoleChanged,
		DisplayName:            "Organization role changed",
		Description:            "Sent after a member's organization role changes.",
		DefaultSubjectTemplate: "Your role in {{organization_name}} changed",
		DefaultTextTemplate:    defaultOrganizationRoleChanged.Text,
		DefaultHTMLTemplate:    defaultOrganizationRoleChanged.HTML,
		AvailableVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariablePreviousRole,
			TemplateVariableRole,
			TemplateVariableOccurredAt,
		},
		RequiredSubjectVariables: []string{
			TemplateVariableOrganizationName,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariablePreviousRole,
			TemplateVariableRole,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOrganizationName: "Acme Inc.",
			TemplateVariablePreviousRole:     "member",
			TemplateVariableRole:             "admin",
			TemplateVariableOccurredAt:       "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateOrganizationOwnershipTransferred,
		DisplayName:            "Organization ownership transferred",
		Description:            "Sent to both parties after organization ownership is transferred.",
		DefaultSubjectTemplate: "Ownership of {{organization_name}} was transferred",
		DefaultTextTemplate:    defaultOrganizationOwnershipTransferred.Text,
		DefaultHTMLTemplate:    defaultOrganizationOwnershipTransferred.HTML,
		AvailableVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariablePreviousOwnerEmail,
			TemplateVariableNewOwnerEmail,
			TemplateVariableOccurredAt,
		},
		RequiredSubjectVariables: []string{
			TemplateVariableOrganizationName,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariablePreviousOwnerEmail,
			TemplateVariableNewOwnerEmail,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOrganizationName:   "Acme Inc.",
			TemplateVariablePreviousOwnerEmail: "previous-owner@example.com",
			TemplateVariableNewOwnerEmail:      "new-owner@example.com",
			TemplateVariableOccurredAt:         "2026-06-24T12:00:00Z",
		},
	},
	{
		Key:                    domain.EmailTemplateOrganizationDeleted,
		DisplayName:            "Organization deleted",
		Description:            "Sent to every member after a team organization is deleted.",
		DefaultSubjectTemplate: "{{organization_name}} was deleted",
		DefaultTextTemplate:    defaultOrganizationDeleted.Text,
		DefaultHTMLTemplate:    defaultOrganizationDeleted.HTML,
		AvailableVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariableOccurredAt,
		},
		RequiredSubjectVariables: []string{
			TemplateVariableOrganizationName,
		},
		RequiredBodyVariables: []string{
			TemplateVariableOrganizationName,
			TemplateVariableOccurredAt,
		},
		SampleData: TemplateData{
			TemplateVariableOrganizationName: "Acme Inc.",
			TemplateVariableOccurredAt:       "2026-06-24T12:00:00Z",
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
