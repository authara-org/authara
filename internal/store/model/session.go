package model

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`
	UpdatedAt time.Time  `gorm:"not null;column:updated_at"`

	UserID string `gorm:"type:uuid;not null;index;column:user_id"`

	RefreshToken string    `gorm:"type:varchar(512);not null;uniqueIndex;column:refresh_token"`
	IssuedAt     time.Time `gorm:"not null;column:issued_at"`
	ExpiresAt    time.Time `gorm:"not null;column:expires_at"`
	Revoked      bool      `gorm:"not null;default:false;column:revoked"`

	UserAgent *string `gorm:"type:varchar(255);column:user_agent"`
}

func (Session) TableName() string {
	return "sessions"
}
