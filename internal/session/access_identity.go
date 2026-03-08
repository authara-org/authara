package session

import (
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/google/uuid"
)

type AccessIdentity struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	Roles     roles.Roles
}
