package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type organizationsResponse struct {
	Organizations []response.Organization `json:"organizations"`
}

type organizationMembersResponse struct {
	Members []organizationMemberDTO `json:"members"`
}

type organizationMemberDTO struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *APIHandler) OrganizationsGet(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		writeOrganizationUnauthorized(w, OrganizationsGetErrors)
		return
	}

	orgs, err := h.Organizations.ListUserOrganizations(r.Context(), userID)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(OrganizationsGetErrors, response.CodeInternalError),
			"Organization error",
		)
		return
	}

	out := make([]response.Organization, 0, len(orgs))
	for _, org := range orgs {
		out = append(out, organizationResponse(org.Organization, org.Membership.Role))
	}
	response.JSON(w, http.StatusOK, organizationsResponse{Organizations: out})
}

func (h *APIHandler) OrganizationCurrentGet(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := httpctx.OrganizationID(r.Context())
	if !ok {
		writeOrganizationUnauthorized(w, OrganizationsGetErrors)
		return
	}
	organizationRole, ok := httpctx.OrganizationRole(r.Context())
	if !ok {
		writeOrganizationUnauthorized(w, OrganizationsGetErrors)
		return
	}

	org, err := h.Organizations.GetOrganization(r.Context(), organizationID)
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		writeOrganizationUnauthorized(w, OrganizationsGetErrors)
		return
	case err != nil:
		response.WriteError(
			w,
			mustRouteError(OrganizationsGetErrors, response.CodeInternalError),
			"Organization error",
		)
		return
	}

	response.JSON(w, http.StatusOK, organizationResponse(org, organizationRole))
}

func (h *APIHandler) OrganizationCurrentMembersGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		writeOrganizationUnauthorized(w, OrganizationMembersGetErrors)
		return
	}
	organizationID, ok := httpctx.OrganizationID(ctx)
	if !ok {
		writeOrganizationUnauthorized(w, OrganizationMembersGetErrors)
		return
	}
	if _, ok := httpctx.OrganizationRole(ctx); !ok {
		writeOrganizationUnauthorized(w, OrganizationMembersGetErrors)
		return
	}

	members, err := h.Organizations.ListCurrentOrganizationMembers(ctx, userID, organizationID)
	switch {
	case errors.Is(err, organization.ErrOrganizationOperationForbidden):
		response.WriteError(
			w,
			mustRouteError(OrganizationMembersGetErrors, response.CodeForbidden),
			"Organization members are not visible.",
		)
		return
	case errors.Is(err, store.ErrOrganizationMembershipNotFound),
		errors.Is(err, store.ErrOrganizationNotFound):
		writeOrganizationUnauthorized(w, OrganizationMembersGetErrors)
		return
	case err != nil:
		response.WriteError(
			w,
			mustRouteError(OrganizationMembersGetErrors, response.CodeInternalError),
			"Organization error",
		)
		return
	}

	out := make([]organizationMemberDTO, 0, len(members))
	for _, member := range members {
		out = append(out, organizationMemberResponse(member))
	}
	response.JSON(w, http.StatusOK, organizationMembersResponse{Members: out})
}

func (h *APIHandler) OrganizationSwitchPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		writeOrganizationUnauthorized(w, OrganizationSwitchPostErrors)
		return
	}
	sessionID, ok := httpctx.SessionID(r.Context())
	if !ok {
		writeOrganizationUnauthorized(w, OrganizationSwitchPostErrors)
		return
	}
	if !h.Organizations.Mode().AllowsOrgSwitching() {
		response.WriteError(
			w,
			mustRouteError(OrganizationSwitchPostErrors, response.CodeForbidden),
			"Organization switching is disabled.",
		)
		return
	}
	organizationID, err := uuid.Parse(chi.URLParam(r, "organizationID"))
	if err != nil || organizationID == uuid.Nil {
		response.WriteError(
			w,
			mustRouteError(OrganizationSwitchPostErrors, response.CodeInvalidRequest),
			"Invalid organization ID.",
		)
		return
	}
	audience, err := redirect.AudienceFromRequest(r)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(OrganizationSwitchPostErrors, response.CodeInvalidRequest),
			"Invalid audience.",
		)
		return
	}

	accessToken, refreshToken, err := h.Session.SwitchSessionOrganization(
		r.Context(),
		userID,
		sessionID,
		organizationID,
		audience,
		time.Now(),
	)
	switch {
	case errors.Is(err, session.ErrInvalidSession):
		writeOrganizationUnauthorized(w, OrganizationSwitchPostErrors)
		return
	case errors.Is(err, session.ErrForbidden),
		errors.Is(err, session.ErrUserDisabled),
		errors.Is(err, session.ErrUserNotAllowed):
		response.WriteError(
			w,
			mustRouteError(OrganizationSwitchPostErrors, response.CodeForbidden),
			"Organization switch forbidden.",
		)
		return
	case err != nil:
		response.WriteError(
			w,
			mustRouteError(OrganizationSwitchPostErrors, response.CodeInternalError),
			"Session error.",
		)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))
	response.JSON(w, http.StatusOK, tokensResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func organizationResponse(org domain.Organization, role domain.OrganizationRole) response.Organization {
	return response.Organization{
		ID:   org.ID.String(),
		Name: org.Name,
		Role: role,
	}
}

func organizationMemberResponse(member domain.OrganizationMember) organizationMemberDTO {
	return organizationMemberDTO{
		UserID:    member.User.ID.String(),
		Email:     member.User.Email,
		Username:  member.User.Username,
		Role:      string(member.Membership.Role),
		CreatedAt: member.Membership.CreatedAt,
	}
}

func writeOrganizationUnauthorized(w http.ResponseWriter, routeErrors map[response.ErrorCode]response.ErrorSpec) {
	response.WriteError(
		w,
		mustRouteError(routeErrors, response.CodeUnauthorized),
		"Unauthorized",
	)
}
