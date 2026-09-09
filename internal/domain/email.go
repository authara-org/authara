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
	EmailTemplateSignupCode         EmailTemplate = "signup_code"
	EmailTemplatePasswordResetCode  EmailTemplate = "password_reset_code"
	EmailTemplateEmailChangeCode    EmailTemplate = "email_change_code"
	EmailTemplateOrganizationInvite EmailTemplate = "organization_invitation"
)

// SupportedEmailTemplates returns every email template understood by Core in
// its stable display order.
func SupportedEmailTemplates() []EmailTemplate {
	return []EmailTemplate{
		EmailTemplateSignupCode,
		EmailTemplatePasswordResetCode,
		EmailTemplateEmailChangeCode,
		EmailTemplateOrganizationInvite,
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
