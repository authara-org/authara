package domain

import (
	"time"

	"github.com/google/uuid"
)

type AppleCredential struct {
	AuthProviderID        uuid.UUID
	EncryptionKeyID       string
	EncryptedRefreshToken []byte
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type AppleTokenRevocation struct {
	ID                    uuid.UUID
	AuthProviderID        *uuid.UUID
	EncryptionContext     uuid.UUID
	EncryptionKeyID       string
	EncryptedRefreshToken []byte
	AttemptCount          int
	NextAttemptAt         time.Time
	LastError             *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
