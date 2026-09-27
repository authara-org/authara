package domain

import (
	"time"

	"github.com/google/uuid"
)

const SecurityEventPasskeyCloneWarning = "passkey.clone_warning"

type SecurityEvent struct {
	ID        uuid.UUID
	CreatedAt time.Time
	Type      string
	UserID    *uuid.UUID
	PasskeyID *uuid.UUID
	Response  string
}
