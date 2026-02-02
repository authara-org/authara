package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	DisabledAt *time.Time

	Email    string
	Username string
}
