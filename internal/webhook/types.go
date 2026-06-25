package webhook

import (
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type EventType string

const (
	EventUserCreated                    EventType = "user.created"
	EventUserUpdated                    EventType = "user.updated"
	EventUserDeleted                    EventType = "user.deleted"
	EventOrganizationCreated            EventType = "organization.created"
	EventOrganizationUpdated            EventType = "organization.updated"
	EventOrganizationDeleted            EventType = "organization.deleted"
	EventOrganizationMembershipCreated  EventType = "organization.membership.created"
	EventOrganizationMembershipUpdated  EventType = "organization.membership.updated"
	EventOrganizationMembershipDeleted  EventType = "organization.membership.deleted"
	EventOrganizationInvitationCreated  EventType = "organization.invitation.created"
	EventOrganizationInvitationAccepted EventType = "organization.invitation.accepted"
	EventOrganizationInvitationRevoked  EventType = "organization.invitation.revoked"
)

var SupportedEventTypes = []EventType{
	EventUserCreated,
	EventUserUpdated,
	EventUserDeleted,
	EventOrganizationCreated,
	EventOrganizationUpdated,
	EventOrganizationDeleted,
	EventOrganizationMembershipCreated,
	EventOrganizationMembershipUpdated,
	EventOrganizationMembershipDeleted,
	EventOrganizationInvitationCreated,
	EventOrganizationInvitationAccepted,
	EventOrganizationInvitationRevoked,
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

type OrganizationData struct {
	OrganizationID  uuid.UUID  `json:"organization_id"`
	Name            string     `json:"name"`
	Kind            string     `json:"kind"`
	CreatedByUserID *uuid.UUID `json:"created_by_user_id"`
}

type OrganizationMembershipData struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`
	Role           string    `json:"role"`
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

type OrganizationInvitationRevokedData struct {
	InvitationID    uuid.UUID  `json:"invitation_id"`
	OrganizationID  uuid.UUID  `json:"organization_id"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	RevokedByUserID *uuid.UUID `json:"revoked_by_user_id"`
	RevokedAt       time.Time  `json:"revoked_at"`
}

func NewUserCreated(userID uuid.UUID, now time.Time) Envelope {
	return newEnvelope(EventUserCreated, now, UserData{UserID: userID})
}

func NewUserUpdated(userID uuid.UUID, now time.Time) Envelope {
	return newEnvelope(EventUserUpdated, now, UserData{UserID: userID})
}

func NewUserDeleted(userID uuid.UUID, now time.Time) Envelope {
	return newEnvelope(EventUserDeleted, now, UserData{UserID: userID})
}

func NewOrganizationCreated(org domain.Organization, now time.Time) Envelope {
	return newEnvelope(EventOrganizationCreated, now, organizationData(org))
}

func NewOrganizationUpdated(org domain.Organization, now time.Time) Envelope {
	return newEnvelope(EventOrganizationUpdated, now, organizationData(org))
}

func NewOrganizationDeleted(org domain.Organization, now time.Time) Envelope {
	return newEnvelope(EventOrganizationDeleted, now, organizationData(org))
}

func NewOrganizationMembershipCreated(membership domain.OrganizationMembership, now time.Time) Envelope {
	return newEnvelope(EventOrganizationMembershipCreated, now, organizationMembershipData(membership))
}

func NewOrganizationMembershipUpdated(membership domain.OrganizationMembership, now time.Time) Envelope {
	return newEnvelope(EventOrganizationMembershipUpdated, now, organizationMembershipData(membership))
}

func NewOrganizationMembershipDeleted(membership domain.OrganizationMembership, now time.Time) Envelope {
	return newEnvelope(EventOrganizationMembershipDeleted, now, organizationMembershipData(membership))
}

func NewOrganizationInvitationCreated(invitation domain.OrganizationInvitation, now time.Time) Envelope {
	return newEnvelope(
		EventOrganizationInvitationCreated,
		now,
		OrganizationInvitationCreatedData{
			InvitationID:    invitation.ID,
			OrganizationID:  invitation.OrganizationID,
			Email:           invitation.Email,
			Role:            string(invitation.Role),
			InvitedByUserID: invitation.InvitedByUserID,
			ExpiresAt:       invitation.ExpiresAt.UTC(),
		},
	)
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

	return newEnvelope(
		EventOrganizationInvitationAccepted,
		now,
		OrganizationInvitationAcceptedData{
			InvitationID:     invitation.ID,
			OrganizationID:   invitation.OrganizationID,
			Email:            invitation.Email,
			Role:             string(invitation.Role),
			AcceptedByUserID: acceptedBy,
			AcceptedAt:       acceptedAt,
		},
	)
}

func NewOrganizationInvitationRevoked(invitation domain.OrganizationInvitation, now time.Time) Envelope {
	revokedAt := now.UTC()
	if invitation.RevokedAt != nil {
		revokedAt = invitation.RevokedAt.UTC()
	}

	return newEnvelope(
		EventOrganizationInvitationRevoked,
		now,
		OrganizationInvitationRevokedData{
			InvitationID:    invitation.ID,
			OrganizationID:  invitation.OrganizationID,
			Email:           invitation.Email,
			Role:            string(invitation.Role),
			RevokedByUserID: invitation.RevokedByUserID,
			RevokedAt:       revokedAt,
		},
	)
}

func newEnvelope(eventType EventType, now time.Time, data any) Envelope {
	return Envelope{
		ID:        uuid.NewString(),
		Type:      eventType,
		CreatedAt: now.UTC(),
		Data:      data,
	}
}

func organizationData(org domain.Organization) OrganizationData {
	return OrganizationData{
		OrganizationID:  org.ID,
		Name:            org.Name,
		Kind:            string(org.Kind),
		CreatedByUserID: org.CreatedByUserID,
	}
}

func organizationMembershipData(membership domain.OrganizationMembership) OrganizationMembershipData {
	return OrganizationMembershipData{
		OrganizationID: membership.OrganizationID,
		UserID:         membership.UserID,
		Role:           string(membership.Role),
	}
}
