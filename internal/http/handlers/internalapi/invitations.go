package internalapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

type Handler struct {
	Organizations *organization.Service
}

func New(organizations *organization.Service) *Handler {
	return &Handler{
		Organizations: organizations,
	}
}

type createInvitationRequest struct {
	ActorUserID string `json:"actor_user_id"`
	Email       string `json:"email"`
}

type invitationResponse struct {
	Invitation invitationDTO `json:"invitation"`
}

type invitationDTO struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	ExpiresAt      string    `json:"expires_at"`
	InviteURL      string    `json:"invite_url,omitempty"`
}

func (h *Handler) CreateOrganizationInvitation(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := parseUUIDParam(w, r, "organizationID", CreateOrganizationInvitationErrors)
	if !ok {
		return
	}

	var req createInvitationRequest
	if !readInternalJSON(w, r, &req, CreateOrganizationInvitationErrors) {
		return
	}

	actorUserID, ok := parseUUIDString(w, req.ActorUserID, "Invalid actor_user_id", CreateOrganizationInvitationErrors)
	if !ok {
		return
	}

	now := time.Now().UTC()
	out, err := h.Organizations.CreateInvitation(r.Context(), organization.CreateInvitationInput{
		OrganizationID: organizationID,
		ActorUserID:    actorUserID,
		Email:          req.Email,
		Now:            now,
	})
	if err != nil {
		h.writeCreateInvitationError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, invitationResponse{
		Invitation: toInvitationDTO(out.Invitation, out.InviteURL, now),
	})
}

func (h *Handler) writeCreateInvitationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		writeRouteError(w, CreateOrganizationInvitationErrors, codeOrganizationNotFound, "Organization not found")
	case errors.Is(err, organization.ErrOrganizationActorNotMember):
		writeRouteError(w, CreateOrganizationInvitationErrors, codeActorNotMember, "Actor is not a member of this organization")
	case errors.Is(err, organization.ErrOrganizationInviteForbidden):
		writeRouteError(w, CreateOrganizationInvitationErrors, codeActorNotAllowed, "Actor is not allowed to invite members")
	case errors.Is(err, organization.ErrOrganizationMemberAlreadyExists):
		writeRouteError(w, CreateOrganizationInvitationErrors, codeAlreadyMember, "User is already a member")
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyPending):
		writeRouteError(w, CreateOrganizationInvitationErrors, codeInvitationAlreadyPending, "Invitation already pending")
	case errors.Is(err, organization.ErrInvalidOrganizationInvitationEmail):
		writeRouteError(w, CreateOrganizationInvitationErrors, response.CodeInvalidRequest, "Invalid invitation request")
	default:
		writeRouteError(w, CreateOrganizationInvitationErrors, response.CodeInternalError, "Internal server error")
	}
}

func toInvitationDTO(invitation domain.OrganizationInvitation, inviteURL string, now time.Time) invitationDTO {
	return invitationDTO{
		ID:             invitation.ID,
		OrganizationID: invitation.OrganizationID,
		Email:          invitation.Email,
		Role:           string(invitation.Role),
		Status:         string(invitation.Status(now)),
		ExpiresAt:      invitation.ExpiresAt.UTC().Format(time.RFC3339),
		InviteURL:      inviteURL,
	}
}
