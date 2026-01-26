package model

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	Name      string     `gorm:"type:text;not null;uniqueIndex;column:name"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`
}

func (Role) TableName() string {
	return "roles"
}
