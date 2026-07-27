package internalapi

import (
	"context"
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *Handler) GetPublicOrganization(ctx context.Context, request contract.GetPublicOrganizationRequestObject) (contract.GetPublicOrganizationResponseObject, error) {
	return h.getOrganization(ctx, request.OrganizationID, GetPublicOrganizationErrors), nil
}

func (h *Handler) UpdatePublicOrganization(ctx context.Context, request contract.UpdatePublicOrganizationRequestObject) (contract.UpdatePublicOrganizationResponseObject, error) {
	if request.Body == nil {
		return routeError(UpdatePublicOrganizationErrors, response.CodeInvalidRequest, "Invalid request body"), nil
	}
	return h.updateOrganization(ctx, request.OrganizationID, request.Body.Name, UpdatePublicOrganizationErrors), nil
}

func (h *Handler) ListPublicOrganizationMembers(ctx context.Context, request contract.ListPublicOrganizationMembersRequestObject) (contract.ListPublicOrganizationMembersResponseObject, error) {
	return h.listOrganizationMembers(ctx, request.OrganizationID, ListPublicOrganizationMembersErrors), nil
}

func (h *Handler) GetPublicOrganizationMember(ctx context.Context, request contract.GetPublicOrganizationMemberRequestObject) (contract.GetPublicOrganizationMemberResponseObject, error) {
	return h.getOrganizationMember(ctx, request.OrganizationID, request.UserID, GetPublicOrganizationMemberErrors), nil
}

func (h *Handler) ListPublicUserMemberships(ctx context.Context, request contract.ListPublicUserMembershipsRequestObject) (contract.ListPublicUserMembershipsResponseObject, error) {
	if currentUserID, publicRequest := httpctx.UserID(ctx); publicRequest && currentUserID != request.UserID {
		return routeError(ListPublicUserMembershipsErrors, response.CodeForbidden, "Organization operation forbidden"), nil
	}
	return h.listUserMemberships(ctx, request.UserID, ListPublicUserMembershipsErrors), nil
}

func (h *Handler) CreateInternalOrganization(ctx context.Context, request contract.CreateInternalOrganizationRequestObject) (contract.CreateInternalOrganizationResponseObject, error) {
	if request.Body == nil {
		return routeError(CreateInternalOrganizationErrors, response.CodeInvalidRequest, "Invalid request body"), nil
	}
	return h.createOrganization(ctx, request.Body.Name, request.Body.CreatedByUserId, CreateInternalOrganizationErrors), nil
}

func (h *Handler) createOrganization(ctx context.Context, name string, createdByUserID openapi_types.UUID, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	org, membership, err := h.Organizations.CreateOrganization(ctx, organization.CreateOrganizationInput{
		Name:            name,
		CreatedByUserID: createdByUserID,
	})
	if err != nil {
		return organizationError(routeErrors, err)
	}
	return contract.JSON(http.StatusCreated, contract.OrganizationWithMembership{
		Organization: toContractOrganization(org),
		Membership:   toContractMembership(membership),
	})
}

func (h *Handler) getOrganization(ctx context.Context, organizationID openapi_types.UUID, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	if _, out, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, routeErrors, false); !ok {
		return out
	}
	org, err := h.Organizations.GetOrganization(ctx, organizationID)
	if err != nil {
		return organizationError(routeErrors, err)
	}
	return contract.JSON(http.StatusOK, contract.OrganizationEnvelope{Organization: toContractOrganization(org)})
}

func (h *Handler) updateOrganization(ctx context.Context, organizationID openapi_types.UUID, name string, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	if _, out, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, routeErrors, true); !ok {
		return out
	}
	org, err := h.Organizations.UpdateOrganization(ctx, organizationID, name)
	if err != nil {
		return organizationError(routeErrors, err)
	}
	return contract.JSON(http.StatusOK, contract.OrganizationEnvelope{Organization: toContractOrganization(org)})
}

func (h *Handler) listOrganizationMembers(ctx context.Context, organizationID openapi_types.UUID, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	publicUserID, out, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, routeErrors, false)
	if !ok {
		return out
	}
	if publicUserID != (openapi_types.UUID{}) && !h.Organizations.Mode().HasVisibleOrganizations() {
		return routeError(routeErrors, response.CodeForbidden, "Organization members are not visible")
	}
	members, err := h.Organizations.ListOrganizationMembers(ctx, organizationID)
	if err != nil {
		return organizationError(routeErrors, err)
	}
	outMembers := make([]contract.OrganizationMember, 0, len(members))
	for _, member := range members {
		outMembers = append(outMembers, toContractOrganizationMember(member))
	}
	return contract.JSON(http.StatusOK, contract.OrganizationMembers{Members: outMembers})
}

func (h *Handler) getOrganizationMember(ctx context.Context, organizationID, userID openapi_types.UUID, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	publicUserID, out, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, routeErrors, false)
	if !ok {
		return out
	}
	if publicUserID != (openapi_types.UUID{}) && !h.Organizations.Mode().HasVisibleOrganizations() {
		return routeError(routeErrors, response.CodeForbidden, "Organization members are not visible")
	}
	member, err := h.Organizations.GetOrganizationMember(ctx, organizationID, userID)
	if err != nil {
		return organizationError(routeErrors, err)
	}
	return contract.JSON(http.StatusOK, contract.OrganizationMemberEnvelope{Member: toContractOrganizationMember(member)})
}

func (h *Handler) listUserMemberships(ctx context.Context, userID openapi_types.UUID, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	memberships, err := h.Organizations.ListUserMemberships(ctx, userID)
	if err != nil {
		return organizationError(routeErrors, err)
	}
	out := make([]contract.MembershipWithOrganization, 0, len(memberships))
	for _, membership := range memberships {
		out = append(out, contract.MembershipWithOrganization{
			Organization: toContractOrganization(membership.Organization),
			Membership:   toContractMembership(membership.Membership),
		})
	}
	return contract.JSON(http.StatusOK, contract.UserMemberships{Memberships: out})
}
