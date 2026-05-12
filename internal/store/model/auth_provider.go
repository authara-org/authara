package model

import (
	"time"

	"github.com/google/uuid"
)

type AuthProvider struct {
	ID uuid.UUID `db:"id"`

	UserID   uuid.UUID `db:"user_id"`
	Provider string    `db:"provider"`

	ProviderUserID *string `db:"provider_user_id"`
	PasswordHash   *string `db:"password_hash"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (AuthProvider) TableName() string {
	return "auth_providers"
}
