package domain

import (
	"time"

	"github.com/google/uuid"
)

type PendingProviderLinkPurpose string

const (
	PendingProviderLinkPurposeAuthenticatedLink PendingProviderLinkPurpose = "authenticated_link"
	PendingProviderLinkPurposeAccountRecovery   PendingProviderLinkPurpose = "account_recovery_link"
)

type PendingProviderLink struct {
	ID        uuid.UUID
	CreatedAt time.Time

	UserID      uuid.UUID
	SessionID   *uuid.UUID
	ChallengeID *uuid.UUID
	Provider    Provider

	ProviderUserID        *string
	ProviderEmail         *string
	ProviderEmailVerified bool
	Purpose               PendingProviderLinkPurpose

	ExpiresAt  time.Time
	ConsumedAt *time.Time
}
