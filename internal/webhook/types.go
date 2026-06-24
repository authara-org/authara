package webhook

import (
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type EventType string

const (
	EventUserCreated                    EventType = "user.created"
	EventUserDeleted                    EventType = "user.deleted"
	EventOrganizationInvitationCreated  EventType = "organization_invitation.created"
	EventOrganizationInvitationAccepted EventType = "organization_invitation.accepted"
	EventOrganizationMembershipCreated  EventType = "organization_membership.created"
)

var SupportedEventTypes = []EventType{
	EventUserCreated,
	EventUserDeleted,
	EventOrganizationInvitationCreated,
	EventOrganizationInvitationAccepted,
	EventOrganizationMembershipCreated,
}

type Envelope struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Data      any       `json:"data"`
}

type UserData struct {
	UserID uuid.UUID `json:"user_id"`
}

type OrganizationInvitationCreatedData struct {
	InvitationID    uuid.UUID  `json:"invitation_id"`
	OrganizationID  uuid.UUID  `json:"organization_id"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	InvitedByUserID *uuid.UUID `json:"invited_by_user_id"`
	ExpiresAt       time.Time  `json:"expires_at"`
}

type OrganizationInvitationAcceptedData struct {
	InvitationID     uuid.UUID `json:"invitation_id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	Email            string    `json:"email"`
	Role             string    `json:"role"`
	AcceptedByUserID uuid.UUID `json:"accepted_by_user_id"`
	AcceptedAt       time.Time `json:"accepted_at"`
}

type OrganizationMembershipCreatedData struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`
	Role           string    `json:"role"`
}

func NewUserCreated(userID uuid.UUID, now time.Time) Envelope {
	return Envelope{
		ID:        uuid.NewString(),
		Type:      EventUserCreated,
		CreatedAt: now.UTC(),
		Data: UserData{
			UserID: userID,
		},
	}
}

func NewUserDeleted(userID uuid.UUID, now time.Time) Envelope {
	return Envelope{
		ID:        uuid.NewString(),
		Type:      EventUserDeleted,
		CreatedAt: now.UTC(),
		Data: UserData{
			UserID: userID,
		},
	}
}

func NewOrganizationInvitationCreated(invitation domain.OrganizationInvitation, now time.Time) Envelope {
	return Envelope{
		ID:        uuid.NewString(),
		Type:      EventOrganizationInvitationCreated,
		CreatedAt: now.UTC(),
		Data: OrganizationInvitationCreatedData{
			InvitationID:    invitation.ID,
			OrganizationID:  invitation.OrganizationID,
			Email:           invitation.Email,
			Role:            string(invitation.Role),
			InvitedByUserID: invitation.InvitedByUserID,
			ExpiresAt:       invitation.ExpiresAt.UTC(),
		},
	}
}

func NewOrganizationInvitationAccepted(invitation domain.OrganizationInvitation, now time.Time) Envelope {
	acceptedAt := now.UTC()
	if invitation.AcceptedAt != nil {
		acceptedAt = invitation.AcceptedAt.UTC()
	}
	acceptedBy := uuid.Nil
	if invitation.AcceptedByUserID != nil {
		acceptedBy = *invitation.AcceptedByUserID
	}

	return Envelope{
		ID:        uuid.NewString(),
		Type:      EventOrganizationInvitationAccepted,
		CreatedAt: now.UTC(),
		Data: OrganizationInvitationAcceptedData{
			InvitationID:     invitation.ID,
			OrganizationID:   invitation.OrganizationID,
			Email:            invitation.Email,
			Role:             string(invitation.Role),
			AcceptedByUserID: acceptedBy,
			AcceptedAt:       acceptedAt,
		},
	}
}

func NewOrganizationMembershipCreated(membership domain.OrganizationMembership, now time.Time) Envelope {
	return Envelope{
		ID:        uuid.NewString(),
		Type:      EventOrganizationMembershipCreated,
		CreatedAt: now.UTC(),
		Data: OrganizationMembershipCreatedData{
			OrganizationID: membership.OrganizationID,
			UserID:         membership.UserID,
			Role:           string(membership.Role),
		},
	}
}
