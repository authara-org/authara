package model

import (
	"time"

	"github.com/google/uuid"
)

type Passkey struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	CredentialID      []byte
	PublicKey         []byte
	AttestationType   string
	AttestationFormat string
	Transport         string
	AAGUID            *uuid.UUID
	SignCount         int64
	CloneWarning      bool
	Name              string
	LastUsedAt        *time.Time

	UserPresent    bool
	UserVerified   bool
	BackupEligible bool
	BackupState    bool
}

func (Passkey) TableName() string {
	return "passkeys"
}

type WebAuthnChallenge struct {
	ID        uuid.UUID
	CreatedAt time.Time

	UserID      *uuid.UUID
	Purpose     string
	Challenge   string
	SessionData []byte
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
}

func (WebAuthnChallenge) TableName() string {
	return "webauthn_challenges"
}
