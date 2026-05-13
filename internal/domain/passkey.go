package domain

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
	Transport         []string
	AAGUID            *uuid.UUID
	SignCount         uint32
	CloneWarning      bool
	Name              string
	LastUsedAt        *time.Time

	UserPresent    bool
	UserVerified   bool
	BackupEligible bool
	BackupState    bool
}

type WebAuthnChallengePurpose string

const (
	WebAuthnChallengePurposeRegistration   WebAuthnChallengePurpose = "registration"
	WebAuthnChallengePurposeAuthentication WebAuthnChallengePurpose = "authentication"
)

type WebAuthnChallenge struct {
	ID        uuid.UUID
	CreatedAt time.Time

	UserID      *uuid.UUID
	Purpose     WebAuthnChallengePurpose
	Challenge   string
	SessionData []byte
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
}
