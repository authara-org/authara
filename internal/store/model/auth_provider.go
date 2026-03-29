package model

import (
	"time"

	"github.com/google/uuid"
)

type AuthProvider struct {
	ID *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`

	UserID   uuid.UUID `gorm:"type:uuid;not null;column:user_id"`
	Provider string    `gorm:"type:varchar(50);not null;column:provider"`

	ProviderUserID *string `gorm:"type:varchar(255);column:provider_user_id"`
	PasswordHash   *string `gorm:"type:varchar(255);column:password_hash"`

	CreatedAt time.Time `gorm:"not null;column:created_at"`
	UpdatedAt time.Time `gorm:"not null;column:updated_at"`
}

func (AuthProvider) TableName() string {
	return "auth_providers"
}
