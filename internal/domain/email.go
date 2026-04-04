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
	EmailTemplateSignupCode EmailTemplate = "signup_code"
	EmailTemplateLoginAlert EmailTemplate = "login_alert"
)

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
