package internalapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	Organizations *organization.Service
	Token         string
}

func New(organizations *organization.Service, token string) *Handler {
	return &Handler{
		Organizations: organizations,
		Token:         token,
	}
}

type createInvitationRequest struct {
	ActorUserID string `json:"actor_user_id"`
	Email       string `json:"email"`
	ReturnTo    string `json:"return_to"`
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
	InviteURL      string    `json:"invite_url"`
}

func (h *Handler) CreateOrganizationInvitation(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Unauthorized")
		return
	}

	organizationID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "organizationID")))
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid organization id")
		return
	}

	var req createInvitationRequest
	defer r.Body.Close()
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid request body")
		return
	}

	actorUserID, err := uuid.Parse(strings.TrimSpace(req.ActorUserID))
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid actor_user_id")
		return
	}

	now := time.Now().UTC()
	out, err := h.Organizations.CreateInvitation(r.Context(), organization.CreateInvitationInput{
		OrganizationID: organizationID,
		ActorUserID:    actorUserID,
		Email:          req.Email,
		ReturnTo:       req.ReturnTo,
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

func (h *Handler) authorized(r *http.Request) bool {
	if strings.TrimSpace(h.Token) == "" {
		return false
	}
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.Token)) == 1
}

func (h *Handler) writeCreateInvitationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		response.ErrorJSON(w, http.StatusNotFound, response.ErrorCode("organization_not_found"), "Organization not found")
	case errors.Is(err, organization.ErrOrganizationActorNotMember):
		response.ErrorJSON(w, http.StatusForbidden, response.ErrorCode("actor_not_member"), "Actor is not a member of this organization")
	case errors.Is(err, organization.ErrOrganizationInviteForbidden):
		response.ErrorJSON(w, http.StatusForbidden, response.ErrorCode("actor_not_allowed"), "Actor is not allowed to invite members")
	case errors.Is(err, organization.ErrOrganizationMemberAlreadyExists):
		response.ErrorJSON(w, http.StatusConflict, response.ErrorCode("already_member"), "User is already a member")
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyPending):
		response.ErrorJSON(w, http.StatusConflict, response.ErrorCode("invitation_already_pending"), "Invitation already pending")
	case errors.Is(err, organization.ErrInvalidOrganizationInvitationEmail):
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid invitation request")
	default:
		response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
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
