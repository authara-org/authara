package model

import (
	"time"

	"github.com/google/uuid"
)

type OrganizationInvitation struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	OrganizationID  uuid.UUID  `db:"organization_id"`
	Email           string     `db:"email"`
	Role            string     `db:"role"`
	TokenHash       string     `db:"token_hash"`
	InvitedByUserID *uuid.UUID `db:"invited_by_user_id"`

	ExpiresAt time.Time `db:"expires_at"`

	AcceptedAt       *time.Time `db:"accepted_at"`
	AcceptedByUserID *uuid.UUID `db:"accepted_by_user_id"`
	RevokedAt        *time.Time `db:"revoked_at"`
	RevokedByUserID  *uuid.UUID `db:"revoked_by_user_id"`
}

func (OrganizationInvitation) TableName() string {
	return "organization_invitations"
}
