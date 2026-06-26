package domain

import (
	"time"

	"github.com/google/uuid"
)

type OrganizationKind string

const (
	OrganizationKindPersonal OrganizationKind = "personal"
	OrganizationKindTeam     OrganizationKind = "team"
)

type OrganizationRole string

const (
	OrganizationRoleOwner  OrganizationRole = "owner"
	OrganizationRoleAdmin  OrganizationRole = "admin"
	OrganizationRoleMember OrganizationRole = "member"
)

type OrganizationInvitationStatus string

const (
	OrganizationInvitationStatusPending  OrganizationInvitationStatus = "pending"
	OrganizationInvitationStatusAccepted OrganizationInvitationStatus = "accepted"
	OrganizationInvitationStatusRevoked  OrganizationInvitationStatus = "revoked"
	OrganizationInvitationStatusExpired  OrganizationInvitationStatus = "expired"
)

type Organization struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	Name            string
	Kind            OrganizationKind
	CreatedByUserID *uuid.UUID
}

type OrganizationMembership struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID

	Role OrganizationRole

	CreatedAt time.Time
	UpdatedAt time.Time
}

type OrganizationMember struct {
	User       User
	Membership OrganizationMembership
}

type OrganizationInvitation struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	OrganizationID  uuid.UUID
	Email           string
	Role            OrganizationRole
	TokenHash       string
	InvitedByUserID *uuid.UUID

	ExpiresAt time.Time

	AcceptedAt       *time.Time
	AcceptedByUserID *uuid.UUID
	RevokedAt        *time.Time
	RevokedByUserID  *uuid.UUID
}

func (i OrganizationInvitation) Status(now time.Time) OrganizationInvitationStatus {
	switch {
	case i.AcceptedAt != nil:
		return OrganizationInvitationStatusAccepted
	case i.RevokedAt != nil:
		return OrganizationInvitationStatusRevoked
	case !i.ExpiresAt.After(now):
		return OrganizationInvitationStatusExpired
	default:
		return OrganizationInvitationStatusPending
	}
}
