package model

import (
	"time"

	"github.com/google/uuid"
)

type OrganizationMembership struct {
	OrganizationID uuid.UUID `db:"organization_id"`
	UserID         uuid.UUID `db:"user_id"`

	Role string `db:"role"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (OrganizationMembership) TableName() string {
	return "organization_memberships"
}
