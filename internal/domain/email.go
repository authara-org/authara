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

type EmailJob struct {
	ID          uuid.UUID
	ChallengeID uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time

	ToEmail       string
	Template      string
	Status        EmailJobStatus
	AttemptCount  int
	LastError     *string
	NextAttemptAt time.Time
	SentAt        *time.Time
}
