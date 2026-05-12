package model

import (
	"time"

	"github.com/google/uuid"
)

type UserRole struct {
	UserID uuid.UUID `db:"user_id"`
	RoleID uuid.UUID `db:"role_id"`

	CreatedAt time.Time `db:"created_at"`
}

func (UserRole) TableName() string {
	return "user_platform_roles"
}
