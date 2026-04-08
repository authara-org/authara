package model

import (
	"time"

	"github.com/google/uuid"
)

type PendingSignupAction struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`

	ChallengeID  uuid.UUID `gorm:"type:uuid;not null;column:challenge_id"`
	Email        string    `gorm:"type:varchar(255);not null;column:email"`
	Username     string    `gorm:"type:varchar(255);not null;column:username"`
	PasswordHash string    `gorm:"type:varchar(255);not null;column:password_hash"`
}

func (PendingSignupAction) TableName() string {
	return "pending_signup_actions"
}

type PendingPasswordReset struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`

	ChallengeID  uuid.UUID `gorm:"type:uuid;not null;column:challenge_id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;column:user_id"`
	PasswordHash string    `gorm:"type:varchar(255);not null;column:password_hash"`
}

func (PendingPasswordReset) TableName() string {
	return "pending_password_resets"
}
