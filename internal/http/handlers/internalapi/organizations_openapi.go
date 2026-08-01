package internalapi

import (
	"context"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *Handler) GetPublicOrganization(ctx context.Context, request contract.GetPublicOrganizationRequestObject) (contract.GetPublicOrganizationResponseObject, error) {
	return h.getOrganization(ctx, request.OrganizationID), nil
}

func (h *Handler) UpdatePublicOrganization(ctx context.Context, request contract.UpdatePublicOrganizationRequestObject) (contract.UpdatePublicOrganizationResponseObject, error) {
	if request.Body == nil {
		return updatePublicOrganizationError(response.CodeInvalidRequest, "Invalid request body"), nil
	}
	return h.updateOrganization(ctx, request.OrganizationID, request.Body.Name), nil
}

func (h *Handler) ListPublicOrganizationMembers(ctx context.Context, request contract.ListPublicOrganizationMembersRequestObject) (contract.ListPublicOrganizationMembersResponseObject, error) {
	return h.listOrganizationMembers(ctx, request.OrganizationID), nil
}

func (h *Handler) GetPublicOrganizationMember(ctx context.Context, request contract.GetPublicOrganizationMemberRequestObject) (contract.GetPublicOrganizationMemberResponseObject, error) {
	return h.getOrganizationMember(ctx, request.OrganizationID, request.UserID), nil
}

func (h *Handler) ListPublicUserMemberships(ctx context.Context, request contract.ListPublicUserMembershipsRequestObject) (contract.ListPublicUserMembershipsResponseObject, error) {
	if currentUserID, publicRequest := httpctx.UserID(ctx); publicRequest && currentUserID != request.UserID {
		return listPublicUserMembershipsError(response.CodeForbidden, "Organization operation forbidden"), nil
	}
	return h.listUserMemberships(ctx, request.UserID), nil
}

func (h *Handler) CreateInternalOrganization(ctx context.Context, request contract.CreateInternalOrganizationRequestObject) (contract.CreateInternalOrganizationResponseObject, error) {
	if request.Body == nil {
		return createInternalOrganizationError(response.CodeInvalidRequest, "Invalid request body"), nil
	}
	return h.createOrganization(ctx, request.Body.Name, request.Body.CreatedByUserId), nil
}

func (h *Handler) createOrganization(ctx context.Context, name string, createdByUserID openapi_types.UUID) contract.CreateInternalOrganizationResponseObject {
	org, membership, err := h.Organizations.CreateOrganization(ctx, organization.CreateOrganizationInput{
		Name:            name,
		CreatedByUserID: createdByUserID,
	})
	if err != nil {
		code, message := organizationError(err)
		return createInternalOrganizationError(code, message)
	}
	return contract.CreateInternalOrganization201JSONResponse(contract.OrganizationWithMembership{
		Organization: toContractOrganization(org),
		Membership:   toContractMembership(membership),
	})
}

func (h *Handler) getOrganization(ctx context.Context, organizationID openapi_types.UUID) contract.GetPublicOrganizationResponseObject {
	if _, code, message, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, false); !ok {
		return getPublicOrganizationError(code, message)
	}
	org, err := h.Organizations.GetOrganization(ctx, organizationID)
	if err != nil {
		code, message := organizationError(err)
		return getPublicOrganizationError(code, message)
	}
	return contract.GetPublicOrganization200JSONResponse(contract.OrganizationEnvelope{Organization: toContractOrganization(org)})
}

func (h *Handler) updateOrganization(ctx context.Context, organizationID openapi_types.UUID, name string) contract.UpdatePublicOrganizationResponseObject {
	if _, code, message, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, true); !ok {
		return updatePublicOrganizationError(code, message)
	}
	org, err := h.Organizations.UpdateOrganization(ctx, organizationID, name)
	if err != nil {
		code, message := organizationError(err)
		return updatePublicOrganizationError(code, message)
	}
	return contract.UpdatePublicOrganization200JSONResponse(contract.OrganizationEnvelope{Organization: toContractOrganization(org)})
}

func (h *Handler) listOrganizationMembers(ctx context.Context, organizationID openapi_types.UUID) contract.ListPublicOrganizationMembersResponseObject {
	publicUserID, code, message, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, false)
	if !ok {
		return listPublicOrganizationMembersError(code, message)
	}
	if publicUserID != (openapi_types.UUID{}) && !h.Organizations.Mode().HasVisibleOrganizations() {
		return listPublicOrganizationMembersError(response.CodeForbidden, "Organization members are not visible")
	}
	members, err := h.Organizations.ListOrganizationMembers(ctx, organizationID)
	if err != nil {
		code, message := organizationError(err)
		return listPublicOrganizationMembersError(code, message)
	}
	outMembers := make([]contract.OrganizationMember, 0, len(members))
	for _, member := range members {
		outMembers = append(outMembers, toContractOrganizationMember(member))
	}
	return contract.ListPublicOrganizationMembers200JSONResponse(contract.OrganizationMembers{Members: outMembers})
}

func (h *Handler) getOrganizationMember(ctx context.Context, organizationID, userID openapi_types.UUID) contract.GetPublicOrganizationMemberResponseObject {
	publicUserID, code, message, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, false)
	if !ok {
		return getPublicOrganizationMemberError(code, message)
	}
	if publicUserID != (openapi_types.UUID{}) && !h.Organizations.Mode().HasVisibleOrganizations() {
		return getPublicOrganizationMemberError(response.CodeForbidden, "Organization members are not visible")
	}
	member, err := h.Organizations.GetOrganizationMember(ctx, organizationID, userID)
	if err != nil {
		code, message := organizationError(err)
		return getPublicOrganizationMemberError(code, message)
	}
	return contract.GetPublicOrganizationMember200JSONResponse(contract.OrganizationMemberEnvelope{Member: toContractOrganizationMember(member)})
}

func (h *Handler) listUserMemberships(ctx context.Context, userID openapi_types.UUID) contract.ListPublicUserMembershipsResponseObject {
	memberships, err := h.Organizations.ListUserMemberships(ctx, userID)
	if err != nil {
		code, message := organizationError(err)
		return listPublicUserMembershipsError(code, message)
	}
	out := make([]contract.MembershipWithOrganization, 0, len(memberships))
	for _, membership := range memberships {
		out = append(out, contract.MembershipWithOrganization{
			Organization: toContractOrganization(membership.Organization),
			Membership:   toContractMembership(membership.Membership),
		})
	}
	return contract.ListPublicUserMemberships200JSONResponse(contract.UserMemberships{Memberships: out})
}
