package model

import (
	"time"

	"github.com/google/uuid"
)

type PendingSignupAction struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`

	ChallengeID  uuid.UUID `db:"challenge_id"`
	Email        string    `db:"email"`
	Username     string    `db:"username"`
	PasswordHash string    `db:"password_hash"`
}

func (PendingSignupAction) TableName() string {
	return "pending_signup_actions"
}

type PendingPasswordReset struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`

	ChallengeID  uuid.UUID `db:"challenge_id"`
	UserID       uuid.UUID `db:"user_id"`
	PasswordHash string    `db:"password_hash"`
}

func (PendingPasswordReset) TableName() string {
	return "pending_password_resets"
}

type PendingEmailChange struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`

	ChallengeID uuid.UUID `db:"challenge_id"`
	UserID      uuid.UUID `db:"user_id"`

	OldEmail string `db:"old_email"`
	NewEmail string `db:"new_email"`
}

func (PendingEmailChange) TableName() string {
	return "pending_email_changes"
}
