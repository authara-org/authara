package internalapi

import (
	"encoding/json"
	"errors"
	"io"
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

const maxInternalJSONBytes = 4096

type capabilitiesResponse struct {
	OrganizationMode          string `json:"organization_mode"`
	HasVisibleOrganizations   bool   `json:"has_visible_organizations"`
	AllowsInvitations         bool   `json:"allows_invitations"`
	AllowsOrgSwitching        bool   `json:"allows_org_switching"`
	AllowsUserCreatedTeamOrgs bool   `json:"allows_user_created_team_orgs"`
	AllowsOrganizationLeave   bool   `json:"allows_organization_leave"`
}

type createOrganizationRequest struct {
	Name            string `json:"name"`
	CreatedByUserID string `json:"created_by_user_id"`
}

type updateOrganizationRequest struct {
	Name string `json:"name"`
}

type revokeInvitationRequest struct {
	RevokedByUserID string `json:"revoked_by_user_id"`
}

type organizationResponse struct {
	Organization organizationDTO `json:"organization"`
	Membership   *membershipDTO  `json:"membership,omitempty"`
}

type organizationsResponse struct {
	Organizations []organizationDTO `json:"organizations"`
}

type membersResponse struct {
	Members []membershipDTO `json:"members"`
}

type memberResponse struct {
	Member membershipDTO `json:"member"`
}

type invitationsResponse struct {
	Invitations []invitationDTO `json:"invitations"`
}

type membershipsResponse struct {
	Memberships []membershipWithOrganizationDTO `json:"memberships"`
}

type organizationDTO struct {
	ID              uuid.UUID  `json:"id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Name            string     `json:"name"`
	Kind            string     `json:"kind"`
	CreatedByUserID *uuid.UUID `json:"created_by_user_id,omitempty"`
}

type membershipDTO struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type membershipWithOrganizationDTO struct {
	Organization organizationDTO `json:"organization"`
	Membership   membershipDTO   `json:"membership"`
}

func (h *Handler) CapabilitiesGet(w http.ResponseWriter, r *http.Request) {
	mode := h.Organizations.Mode()
	response.JSON(w, http.StatusOK, capabilitiesResponse{
		OrganizationMode:          string(mode),
		HasVisibleOrganizations:   mode.HasVisibleOrganizations(),
		AllowsInvitations:         mode.AllowsInvitations(),
		AllowsOrgSwitching:        mode.AllowsOrgSwitching(),
		AllowsUserCreatedTeamOrgs: mode.AllowsUserCreatedTeamOrgs(),
		AllowsOrganizationLeave:   mode.AllowsLeaveOrg(),
	})
}

func (h *Handler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	var req createOrganizationRequest
	if !readInternalJSON(w, r, &req, CreateOrganizationErrors) {
		return
	}
	createdByUserID, ok := parseUUIDString(w, req.CreatedByUserID, "Invalid created_by_user_id", CreateOrganizationErrors)
	if !ok {
		return
	}

	org, membership, err := h.Organizations.CreateOrganization(r.Context(), organization.CreateOrganizationInput{
		Name:            req.Name,
		CreatedByUserID: createdByUserID,
	})
	if err != nil {
		writeInternalOrganizationError(w, CreateOrganizationErrors, err)
		return
	}

	dto := toMembershipDTO(membership)
	response.JSON(w, http.StatusCreated, organizationResponse{
		Organization: toOrganizationDTO(org),
		Membership:   &dto,
	})
}

func (h *Handler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := parseUUIDParam(w, r, "organizationID", OrganizationErrors)
	if !ok {
		return
	}
	org, err := h.Organizations.GetOrganization(r.Context(), organizationID)
	if err != nil {
		writeInternalOrganizationError(w, OrganizationErrors, err)
		return
	}

	response.JSON(w, http.StatusOK, organizationResponse{Organization: toOrganizationDTO(org)})
}

func (h *Handler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := parseUUIDParam(w, r, "organizationID", OrganizationErrors)
	if !ok {
		return
	}
	var req updateOrganizationRequest
	if !readInternalJSON(w, r, &req, OrganizationErrors) {
		return
	}

	org, err := h.Organizations.UpdateOrganization(r.Context(), organizationID, req.Name)
	if err != nil {
		writeInternalOrganizationError(w, OrganizationErrors, err)
		return
	}

	response.JSON(w, http.StatusOK, organizationResponse{Organization: toOrganizationDTO(org)})
}

func (h *Handler) ListOrganizationMembers(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := parseUUIDParam(w, r, "organizationID", OrganizationMembersGetErrors)
	if !ok {
		return
	}
	members, err := h.Organizations.ListOrganizationMembers(r.Context(), organizationID)
	if err != nil {
		writeInternalOrganizationError(w, OrganizationMembersGetErrors, err)
		return
	}

	out := make([]membershipDTO, 0, len(members))
	for _, member := range members {
		out = append(out, toMembershipDTO(member))
	}
	response.JSON(w, http.StatusOK, membersResponse{Members: out})
}

func (h *Handler) GetOrganizationMember(w http.ResponseWriter, r *http.Request) {
	organizationID, userID, ok := parseOrganizationAndUserParams(w, r, OrganizationMemberErrors)
	if !ok {
		return
	}
	member, err := h.Organizations.GetOrganizationMember(r.Context(), organizationID, userID)
	if err != nil {
		writeInternalOrganizationError(w, OrganizationMemberErrors, err)
		return
	}

	response.JSON(w, http.StatusOK, memberResponse{Member: toMembershipDTO(member)})
}

func (h *Handler) ListOrganizationInvitations(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := parseUUIDParam(w, r, "organizationID", OrganizationInvitationsGetErrors)
	if !ok {
		return
	}
	now := time.Now().UTC()
	invitations, err := h.Organizations.ListInvitations(r.Context(), organizationID)
	if err != nil {
		writeInternalOrganizationError(w, OrganizationInvitationsGetErrors, err)
		return
	}

	out := make([]invitationDTO, 0, len(invitations))
	for _, invitation := range invitations {
		out = append(out, toInvitationDTO(invitation, "", now))
	}
	response.JSON(w, http.StatusOK, invitationsResponse{Invitations: out})
}

func (h *Handler) GetOrganizationInvitation(w http.ResponseWriter, r *http.Request) {
	organizationID, invitationID, ok := parseOrganizationAndInvitationParams(w, r, OrganizationInvitationGetErrors)
	if !ok {
		return
	}
	preview, err := h.Organizations.InvitationByOrganizationAndID(r.Context(), organizationID, invitationID)
	if err != nil {
		writeInternalOrganizationError(w, OrganizationInvitationGetErrors, err)
		return
	}

	response.JSON(w, http.StatusOK, invitationResponse{Invitation: toInvitationDTO(preview.Invitation, "", time.Now().UTC())})
}

func (h *Handler) RevokeOrganizationInvitation(w http.ResponseWriter, r *http.Request) {
	organizationID, invitationID, ok := parseOrganizationAndInvitationParams(w, r, RevokeOrganizationInvitationErrors)
	if !ok {
		return
	}

	var req revokeInvitationRequest
	if !readOptionalInternalJSON(w, r, &req, RevokeOrganizationInvitationErrors) {
		return
	}
	var revokedBy *uuid.UUID
	if strings.TrimSpace(req.RevokedByUserID) != "" {
		id, ok := parseUUIDString(w, req.RevokedByUserID, "Invalid revoked_by_user_id", RevokeOrganizationInvitationErrors)
		if !ok {
			return
		}
		revokedBy = &id
	}

	now := time.Now().UTC()
	invitation, err := h.Organizations.RevokeInvitation(r.Context(), organization.RevokeInvitationInput{
		OrganizationID:  organizationID,
		InvitationID:    invitationID,
		RevokedByUserID: revokedBy,
		Now:             now,
	})
	if err != nil {
		writeInternalOrganizationError(w, RevokeOrganizationInvitationErrors, err)
		return
	}

	response.JSON(w, http.StatusOK, invitationResponse{Invitation: toInvitationDTO(invitation, "", now)})
}

func (h *Handler) ListUserMemberships(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUIDParam(w, r, "userID", UserMembershipsGetErrors)
	if !ok {
		return
	}
	memberships, err := h.Organizations.ListUserMemberships(r.Context(), userID)
	if err != nil {
		writeInternalOrganizationError(w, UserMembershipsGetErrors, err)
		return
	}

	out := make([]membershipWithOrganizationDTO, 0, len(memberships))
	for _, membership := range memberships {
		out = append(out, membershipWithOrganizationDTO{
			Organization: toOrganizationDTO(membership.Organization),
			Membership:   toMembershipDTO(membership.Membership),
		})
	}
	response.JSON(w, http.StatusOK, membershipsResponse{Memberships: out})
}

func readInternalJSON(w http.ResponseWriter, r *http.Request, dst any, routeErrors map[response.ErrorCode]response.ErrorSpec) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxInternalJSONBytes)).Decode(dst); err != nil {
		writeRouteError(w, routeErrors, response.CodeInvalidRequest, "Invalid request body")
		return false
	}
	return true
}

func readOptionalInternalJSON(w http.ResponseWriter, r *http.Request, dst any, routeErrors map[response.ErrorCode]response.ErrorSpec) bool {
	defer r.Body.Close()
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxInternalJSONBytes)).Decode(dst)
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}
	writeRouteError(w, routeErrors, response.CodeInvalidRequest, "Invalid request body")
	return false
}

func parseOrganizationAndUserParams(w http.ResponseWriter, r *http.Request, routeErrors map[response.ErrorCode]response.ErrorSpec) (uuid.UUID, uuid.UUID, bool) {
	organizationID, ok := parseUUIDParam(w, r, "organizationID", routeErrors)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	userID, ok := parseUUIDParam(w, r, "userID", routeErrors)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return organizationID, userID, true
}

func parseOrganizationAndInvitationParams(w http.ResponseWriter, r *http.Request, routeErrors map[response.ErrorCode]response.ErrorSpec) (uuid.UUID, uuid.UUID, bool) {
	organizationID, ok := parseUUIDParam(w, r, "organizationID", routeErrors)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	invitationID, ok := parseUUIDParam(w, r, "invitationID", routeErrors)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return organizationID, invitationID, true
}

func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string, routeErrors map[response.ErrorCode]response.ErrorSpec) (uuid.UUID, bool) {
	return parseUUIDString(w, chi.URLParam(r, name), "Invalid "+strings.TrimSuffix(name, "ID")+" id", routeErrors)
}

func parseUUIDString(w http.ResponseWriter, raw string, message string, routeErrors map[response.ErrorCode]response.ErrorSpec) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || id == uuid.Nil {
		writeRouteError(w, routeErrors, response.CodeInvalidRequest, message)
		return uuid.Nil, false
	}
	return id, true
}

func writeInternalOrganizationError(w http.ResponseWriter, routeErrors map[response.ErrorCode]response.ErrorSpec, err error) {
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		writeRouteError(w, routeErrors, codeOrganizationNotFound, "Organization not found")
	case errors.Is(err, store.ErrOrganizationMembershipNotFound):
		writeRouteError(w, routeErrors, codeMembershipNotFound, "Membership not found")
	case errors.Is(err, store.ErrOrganizationInvitationNotFound):
		writeRouteError(w, routeErrors, codeInvitationNotFound, "Invitation not found")
	case errors.Is(err, store.ErrUserNotFound):
		writeRouteError(w, routeErrors, codeUserNotFound, "User not found")
	case errors.Is(err, store.ErrInvalidOrganizationName),
		errors.Is(err, organization.ErrInvalidOrganizationRole):
		writeRouteError(w, routeErrors, response.CodeInvalidRequest, "Invalid organization request")
	case errors.Is(err, organization.ErrOrganizationOperationForbidden),
		errors.Is(err, organization.ErrOrganizationInviteForbidden):
		writeRouteError(w, routeErrors, response.CodeForbidden, "Organization operation forbidden")
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted):
		writeRouteError(w, routeErrors, codeInvitationAlreadyAccepted, "Invitation already accepted")
	case errors.Is(err, organization.ErrOrganizationInvitationRevoked):
		writeRouteError(w, routeErrors, codeInvitationRevoked, "Invitation already revoked")
	case errors.Is(err, organization.ErrOrganizationInvitationExpired):
		writeRouteError(w, routeErrors, codeInvitationExpired, "Invitation expired")
	default:
		writeRouteError(w, routeErrors, response.CodeInternalError, "Internal server error")
	}
}

func writeRouteError(w http.ResponseWriter, routeErrors map[response.ErrorCode]response.ErrorSpec, code response.ErrorCode, message string) {
	response.WriteError(w, mustRouteError(routeErrors, code), message)
}

func toOrganizationDTO(org domain.Organization) organizationDTO {
	return organizationDTO{
		ID:              org.ID,
		CreatedAt:       org.CreatedAt,
		UpdatedAt:       org.UpdatedAt,
		Name:            org.Name,
		Kind:            string(org.Kind),
		CreatedByUserID: org.CreatedByUserID,
	}
}

func toMembershipDTO(membership domain.OrganizationMembership) membershipDTO {
	return membershipDTO{
		OrganizationID: membership.OrganizationID,
		UserID:         membership.UserID,
		Role:           string(membership.Role),
		CreatedAt:      membership.CreatedAt,
		UpdatedAt:      membership.UpdatedAt,
	}
}
