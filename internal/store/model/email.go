package model

import (
	"time"

	"github.com/google/uuid"
)

type EmailJob struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`
	UpdatedAt time.Time  `gorm:"not null;column:updated_at"`

	ChallengeID  *uuid.UUID `gorm:"type:uuid;column:challenge_id"`
	ToEmail      string     `gorm:"type:varchar(255);not null;column:to_email"`
	Template     string     `gorm:"type:varchar(64);not null;column:template"`
	TemplateData []byte     `gorm:"type:jsonb;column:template_data"`
	Status       string     `gorm:"type:varchar(32);not null;index;column:status"`

	AttemptCount        int        `gorm:"not null;column:attempt_count"`
	NextAttemptAt       time.Time  `gorm:"not null;column:next_attempt_at"`
	ProcessingStartedAt *time.Time `gorm:"column:processing_started_at"`
	LastError           *string    `gorm:"type:text;column:last_error"`
	SentAt              *time.Time `gorm:"column:sent_at"`
}

func (EmailJob) TableName() string {
	return "email_jobs"
}
