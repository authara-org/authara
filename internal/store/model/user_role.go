package model

import "github.com/google/uuid"

type UserRole struct {
	UserID uuid.UUID `gorm:"type:uuid;not null;column:user_id;primaryKey"`
	RoleID uuid.UUID `gorm:"type:uuid;not null;column:role_id;primaryKey"`
}

func (UserRole) TableName() string {
	return "user_roles"
}
