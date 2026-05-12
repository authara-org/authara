package model

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

func (Role) TableName() string {
	return "platform_roles"
}
