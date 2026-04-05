package model

import (
	"time"

	"github.com/google/uuid"
)

type UserRole struct {
	UserID uuid.UUID `gorm:"type:uuid;not null;column:user_id;primaryKey"`
	RoleID uuid.UUID `gorm:"type:uuid;not null;column:role_id;primaryKey"`

	CreatedAt time.Time `gorm:"not null;column:created_at"`
}

func (UserRole) TableName() string {
	return "user_platform_roles"
}
