package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *APIHandler) ListCurrentUserOrganizations(ctx context.Context, _ contract.ListCurrentUserOrganizationsRequestObject) (contract.ListCurrentUserOrganizationsResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return listCurrentUserOrganizationsError(responseCodeUnauthorized(), "Unauthorized"), nil
	}
	orgs, err := h.Organizations.ListUserOrganizations(ctx, userID)
	if err != nil {
		return listCurrentUserOrganizationsError(responseCodeInternalError(), "Organization error"), nil
	}
	out := make([]contract.OrganizationSummary, 0, len(orgs))
	for _, org := range orgs {
		out = append(out, toContractOrganizationSummary(org.Organization, org.Membership.Role))
	}
	return contract.ListCurrentUserOrganizations200JSONResponse(contract.OrganizationSummaries{Organizations: out}), nil
}

func (h *APIHandler) GetCurrentOrganization(ctx context.Context, _ contract.GetCurrentOrganizationRequestObject) (contract.GetCurrentOrganizationResponseObject, error) {
	organizationID, role, code, message, ok := currentOrganization(ctx)
	if !ok {
		return getCurrentOrganizationError(code, message), nil
	}
	org, err := h.Organizations.GetOrganization(ctx, organizationID)
	if err != nil {
		return getCurrentOrganizationError(responseCodeUnauthorized(), "Unauthorized"), nil
	}
	return contract.GetCurrentOrganization200JSONResponse(toContractOrganizationSummary(org, role)), nil
}

func (h *APIHandler) ListCurrentOrganizationMembers(ctx context.Context, _ contract.ListCurrentOrganizationMembersRequestObject) (contract.ListCurrentOrganizationMembersResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return listCurrentOrganizationMembersError(responseCodeUnauthorized(), "Unauthorized"), nil
	}
	organizationID, _, code, message, ok := currentOrganization(ctx)
	if !ok {
		return listCurrentOrganizationMembersError(code, message), nil
	}
	members, err := h.Organizations.ListCurrentOrganizationMembers(ctx, userID, organizationID)
	switch {
	case errors.Is(err, organization.ErrOrganizationOperationForbidden):
		return listCurrentOrganizationMembersError(responseCodeForbidden(), "Organization members are not visible."), nil
	case errors.Is(err, store.ErrOrganizationMembershipNotFound),
		errors.Is(err, store.ErrOrganizationNotFound):
		return listCurrentOrganizationMembersError(responseCodeUnauthorized(), "Unauthorized"), nil
	case err != nil:
		return listCurrentOrganizationMembersError(responseCodeInternalError(), "Organization error"), nil
	}
	outMembers := make([]contract.CurrentOrganizationMember, 0, len(members))
	for _, member := range members {
		outMembers = append(outMembers, contract.CurrentOrganizationMember{
			UserId:    member.User.ID,
			Email:     openapi_types.Email(member.User.Email),
			Username:  member.User.Username,
			Role:      contract.OrganizationRole(member.Membership.Role),
			CreatedAt: member.Membership.CreatedAt,
		})
	}
	return contract.ListCurrentOrganizationMembers200JSONResponse(contract.CurrentOrganizationMembers{Members: outMembers}), nil
}

func (h *APIHandler) SwitchOrganization(ctx context.Context, request contract.SwitchOrganizationRequestObject) (contract.SwitchOrganizationResponseObject, error) {
	_, ok := contractRequest(ctx)
	if !ok {
		return switchOrganizationError(responseCodeInternalError(), "API contract error."), nil
	}
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return switchOrganizationError(responseCodeUnauthorized(), "Unauthorized"), nil
	}
	sessionID, ok := httpctx.SessionID(ctx)
	if !ok {
		return switchOrganizationError(responseCodeUnauthorized(), "Unauthorized"), nil
	}
	if !h.Organizations.Mode().AllowsOrgSwitching() {
		return switchOrganizationError(responseCodeForbidden(), "Organization switching is disabled."), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	accessToken, refreshToken, err := h.Session.SwitchSessionOrganization(ctx, userID, sessionID, request.OrganizationID, audience, time.Now())
	switch {
	case errors.Is(err, session.ErrInvalidSession):
		return switchOrganizationError(responseCodeUnauthorized(), "Unauthorized"), nil
	case errors.Is(err, session.ErrForbidden),
		errors.Is(err, session.ErrUserDisabled),
		errors.Is(err, session.ErrUserNotAllowed):
		return switchOrganizationError(responseCodeForbidden(), "Organization switch forbidden."), nil
	case err != nil:
		return switchOrganizationError(responseCodeInternalError(), "Session error."), nil
	}
	header := make(http.Header)
	session.SetAccessToken(contract.HeaderWriter(header), accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(contract.HeaderWriter(header), refreshToken, int(h.RefreshTTL.Seconds()))
	return contract.SwitchOrganization200HeadersResponse{
		Header: header,
		Body:   contract.Tokens{AccessToken: accessToken, RefreshToken: refreshToken},
	}, nil
}
