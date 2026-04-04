package model

import (
	"time"

	"github.com/google/uuid"
)

type Challenge struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`
	UpdatedAt time.Time  `gorm:"not null;column:updated_at"`

	Purpose string `gorm:"type:varchar(64);not null;column:purpose"`
	Email   string `gorm:"type:varchar(255);not null;column:email"`

	ExpiresAt    time.Time  `gorm:"not null;column:expires_at"`
	ConsumedAt   *time.Time `gorm:"column:consumed_at"`
	AttemptCount int        `gorm:"not null;column:attempt_count"`
	MaxAttempts  int        `gorm:"not null;column:max_attempts"`

	ResendCount int        `gorm:"not null;column:resend_count"`
	MaxResends  int        `gorm:"not null;column:max_resends"`
	LastSentAt  *time.Time `gorm:"column:last_sent_at"`
}

func (Challenge) TableName() string {
	return "challenges"
}

type VerificationCode struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`

	ChallengeID uuid.UUID `gorm:"type:uuid;not null;column:challenge_id"`
	CodeHash    string    `gorm:"type:varchar(255);not null;column:code_hash"`
	ExpiresAt   time.Time `gorm:"not null;column:expires_at"`
}

func (VerificationCode) TableName() string {
	return "verification_codes"
}
