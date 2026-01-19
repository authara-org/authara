package model

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`
	UpdatedAt time.Time  `gorm:"not null;column:updated_at"`

	UserID uuid.UUID `gorm:"type:uuid;not null;index;column:user_id"`

	ExpiresAt time.Time  `gorm:"not null;column:expires_at"`
	RevokedAt *time.Time `gorm:"column:revoked_at"`

	UserAgent string `gorm:"type:varchar(255);not null;column:user_agent"`
}

func (Session) TableName() string {
	return "sessions"
}
