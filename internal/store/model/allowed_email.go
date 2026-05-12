package model

import (
	"time"

	"github.com/google/uuid"
)

type AllowedEmail struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Email     string    `db:"email"`
}

func (AllowedEmail) TableName() string {
	return "allowed_emails"
}
