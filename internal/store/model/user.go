package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	DisabledAt *time.Time `db:"disabled_at"`

	Username           string `db:"username"`
	UsernameNormalized string `db:"username_normalized"`
	Email              string `db:"email"`
}

func (User) TableName() string {
	return "users"
}
