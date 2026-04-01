package domain

import (
	"time"

	"github.com/google/uuid"
)

type PendingActionType string

const (
	PendingActionSignup        PendingActionType = "signup"
	PendingActionPasswordReset PendingActionType = "password_reset"
	PendingActionEmailChange   PendingActionType = "email_change"
)

type PendingAccountCreation struct {
	ID          uuid.UUID
	ChallangeID uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time

	Email        string
	Username     string
	PasswordHash string
}
