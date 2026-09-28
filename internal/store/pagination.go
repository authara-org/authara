package store

import (
	"time"

	"github.com/google/uuid"
)

// ListCursor is the decoded keyset boundary for an ordered collection query.
type ListCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}
