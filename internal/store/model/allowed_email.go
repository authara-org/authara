package model

import (
	"time"

	"github.com/google/uuid"
)

type AllowedEmail struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;colum:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`
	UpdatedAt time.Time  `gorm:"not null;column:updated_at"`
	Email     string     `gorm:"type:varchar();not null;column:email"`
}

func (AllowedEmail) TableName() string {
	return "allowed_emails"
}
