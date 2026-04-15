package model

import (
	"time"

	"github.com/google/uuid"
)

type PendingProviderLink struct {
	ID *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`

	UserID    uuid.UUID `gorm:"type:uuid;not null;column:user_id"`
	SessionID uuid.UUID `gorm:"type:uuid;not null;column:session_id"`
	Provider  string    `gorm:"type:varchar(50);not null;column:provider"`

	ExpiresAt  time.Time  `gorm:"not null;column:expires_at"`
	ConsumedAt *time.Time `gorm:"column:consumed_at"`

	CreatedAt time.Time `gorm:"not null;column:created_at"`
}

func (PendingProviderLink) TableName() string {
	return "pending_provider_links"
}
