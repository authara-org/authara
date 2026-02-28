package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        *uuid.UUID `gorm:"type:uuid;primaryKey;column:id;default:gen_random_uuid()"`
	CreatedAt time.Time  `gorm:"not null;column:created_at"`
	UpdatedAt time.Time  `gorm:"not null;column:updated_at"`

	DisabledAt *time.Time `gorm:"column:disabled_at"`

	Username           string `gorm:"type:varchar(255);not null;column:username"`
	UsernameNormalized string `gorm:"type:varchar(255);not null;column:username_normalized"`
	Email              string `gorm:"type:varchar(255);not null;uniqueIndex;column:email"`
}

func (User) TableName() string {
	return "users"
}
