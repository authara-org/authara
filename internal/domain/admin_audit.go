package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AdminAuditEvent struct {
	ID uuid.UUID

	CreatedAt time.Time

	ActorUserID *uuid.UUID
	Action      string

	TargetUserID *uuid.UUID
	TargetEmail  *string
	Metadata     json.RawMessage

	IP        *string
	UserAgent *string
}
