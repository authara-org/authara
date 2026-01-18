package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	// ID is nil only before persistence.
	// After loading or creating a user, ID is always non-nil.
	ID        *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	Email    string
	Username string
}
