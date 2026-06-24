package model

import (
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	Name            string     `db:"name"`
	Kind            string     `db:"kind"`
	CreatedByUserID *uuid.UUID `db:"created_by_user_id"`
}

func (Organization) TableName() string {
	return "organizations"
}
