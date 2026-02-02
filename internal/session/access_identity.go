package session

import (
	"github.com/alexlup06-authgate/authgate/internal/session/roles"
	"github.com/google/uuid"
)

type AccessIdentity struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	Roles     roles.Roles
}
