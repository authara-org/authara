package model

import (
	"time"

	"github.com/google/uuid"
)

type EmailJob struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	ChallengeID  *uuid.UUID `db:"challenge_id"`
	ToEmail      string     `db:"to_email"`
	Template     string     `db:"template"`
	TemplateData []byte     `db:"template_data"`
	Status       string     `db:"status"`

	AttemptCount        int        `db:"attempt_count"`
	NextAttemptAt       time.Time  `db:"next_attempt_at"`
	ProcessingStartedAt *time.Time `db:"processing_started_at"`
	LastError           *string    `db:"last_error"`
	SentAt              *time.Time `db:"sent_at"`
}

func (EmailJob) TableName() string {
	return "email_jobs"
}
