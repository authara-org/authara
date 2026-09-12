package runtime

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Key string

var ErrRevisionConflict = errors.New("runtime setting revision conflict")

type PersistedOverride struct {
	Key             Key
	Value           json.RawMessage
	Revision        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	UpdatedByUserID *uuid.UUID
}

type PersistedState struct {
	Revision  int64
	Overrides []PersistedOverride
}

type Mutation struct {
	Key                   Key
	Value                 json.RawMessage
	ExpectedRevision      int64
	ExpectedStateRevision int64
	ActorUserID           uuid.UUID
	AuditMetadata         json.RawMessage
}
