package domain

import (
	"time"

	"github.com/google/uuid"
)

type EmailJobStatus string

const (
	EmailJobStatusPending    EmailJobStatus = "pending"
	EmailJobStatusProcessing EmailJobStatus = "processing"
	EmailJobStatusSent       EmailJobStatus = "sent"
	EmailJobStatusFailed     EmailJobStatus = "failed"
)

type EmailTemplate string

const (
	EmailTemplateSignupCode                       EmailTemplate = "signup_code"
	EmailTemplatePasswordResetCode                EmailTemplate = "password_reset_code"
	EmailTemplateEmailChangeCode                  EmailTemplate = "email_change_code"
	EmailTemplateOrganizationInvite               EmailTemplate = "organization_invitation"
	EmailTemplateAccountCreated                   EmailTemplate = "account_created"
	EmailTemplateNewSignIn                        EmailTemplate = "new_sign_in"
	EmailTemplateAuthMethodAdded                  EmailTemplate = "auth_method_added"
	EmailTemplateAuthMethodRemoved                EmailTemplate = "auth_method_removed"
	EmailTemplatePasswordChanged                  EmailTemplate = "password_changed"
	EmailTemplateEmailChangedOldAddress           EmailTemplate = "email_changed_old_address"
	EmailTemplateEmailChangedNewAddress           EmailTemplate = "email_changed_new_address"
	EmailTemplateAccountDisabled                  EmailTemplate = "account_disabled"
	EmailTemplateAccountEnabled                   EmailTemplate = "account_enabled"
	EmailTemplateAdminAccessChanged               EmailTemplate = "admin_access_changed"
	EmailTemplateOrganizationInvitationAccepted   EmailTemplate = "organization_invitation_accepted"
	EmailTemplateOrganizationInvitationRevoked    EmailTemplate = "organization_invitation_revoked"
	EmailTemplateOrganizationMembershipRemoved    EmailTemplate = "organization_membership_removed"
	EmailTemplateOrganizationRoleChanged          EmailTemplate = "organization_role_changed"
	EmailTemplateOrganizationOwnershipTransferred EmailTemplate = "organization_ownership_transferred"
	EmailTemplateOrganizationDeleted              EmailTemplate = "organization_deleted"
)

// SupportedEmailTemplates returns every email template understood by Core in
// its stable display order.
func SupportedEmailTemplates() []EmailTemplate {
	return []EmailTemplate{
		EmailTemplateSignupCode,
		EmailTemplatePasswordResetCode,
		EmailTemplateEmailChangeCode,
		EmailTemplateOrganizationInvite,
		EmailTemplateAccountCreated,
		EmailTemplateNewSignIn,
		EmailTemplateAuthMethodAdded,
		EmailTemplateAuthMethodRemoved,
		EmailTemplatePasswordChanged,
		EmailTemplateEmailChangedOldAddress,
		EmailTemplateEmailChangedNewAddress,
		EmailTemplateAccountDisabled,
		EmailTemplateAccountEnabled,
		EmailTemplateAdminAccessChanged,
		EmailTemplateOrganizationInvitationAccepted,
		EmailTemplateOrganizationInvitationRevoked,
		EmailTemplateOrganizationMembershipRemoved,
		EmailTemplateOrganizationRoleChanged,
		EmailTemplateOrganizationOwnershipTransferred,
		EmailTemplateOrganizationDeleted,
	}
}

// EmailTemplateOverride is the persisted operator customization for one
// Core-owned email template. A missing override means the built-in template is
// effective.
type EmailTemplateOverride struct {
	Template EmailTemplate

	CreatedAt time.Time
	UpdatedAt time.Time

	SubjectTemplate string
	TextTemplate    string
	HTMLTemplate    string
	Revision        int64
	UpdatedByUserID *uuid.UUID
}

// EmailTemplateVersion is an immutable snapshot created whenever an operator
// successfully saves or restores a customized template.
type EmailTemplateVersion struct {
	Template EmailTemplate
	Version  int64

	CreatedAt time.Time

	SubjectTemplate string
	TextTemplate    string
	HTMLTemplate    string
	CreatedByUserID *uuid.UUID
}

type EmailJob struct {
	ID          uuid.UUID
	ChallengeID *uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time

	ToEmail             string
	Template            EmailTemplate
	TemplateData        []byte
	Status              EmailJobStatus
	AttemptCount        int
	ProcessingStartedAt *time.Time
	LastError           *string
	NextAttemptAt       time.Time
	SentAt              *time.Time
}
