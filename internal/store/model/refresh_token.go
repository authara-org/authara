package model

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`

	SessionID uuid.UUID `gorm:"type:uuid;not null;index;column:session_id"`

	TokenHash string `gorm:"type:varchar(512);not null;uniqueIndex;column:token_hash"`

	ExpiresAt  time.Time  `gorm:"not null;column:expires_at"`
	ConsumedAt *time.Time `gorm:"column:consumed_at"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
