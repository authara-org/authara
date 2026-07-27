package internalapi

import (
	"context"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/store"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *Handler) contractAuthorizePublicOrganization(
	ctx context.Context,
	organizationID openapi_types.UUID,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
	managerOnly bool,
) (openapi_types.UUID, contract.Response, bool) {
	userID, publicRequest := httpctx.UserID(ctx)
	if !publicRequest {
		return openapi_types.UUID{}, contract.Response{}, true
	}
	currentOrganizationID, ok := httpctx.OrganizationID(ctx)
	if !ok || currentOrganizationID != organizationID {
		return openapi_types.UUID{}, routeError(routeErrors, response.CodeForbidden, "Organization operation forbidden"), false
	}
	membership, err := h.Organizations.RequireMembership(ctx, userID, organizationID)
	if errors.Is(err, store.ErrOrganizationMembershipNotFound) {
		return openapi_types.UUID{}, routeError(routeErrors, response.CodeForbidden, "Organization operation forbidden"), false
	}
	if err != nil {
		return openapi_types.UUID{}, routeError(routeErrors, response.CodeInternalError, "Internal server error"), false
	}
	if managerOnly && membership.Role != domain.OrganizationRoleOwner && membership.Role != domain.OrganizationRoleAdmin {
		return openapi_types.UUID{}, routeError(routeErrors, response.CodeForbidden, "Organization operation forbidden"), false
	}
	return userID, contract.Response{}, true
}

func organizationError(routeErrors map[response.ErrorCode]response.ErrorSpec, err error) contract.Response {
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		return routeError(routeErrors, codeOrganizationNotFound, "Organization not found")
	case errors.Is(err, store.ErrOrganizationMembershipNotFound):
		return routeError(routeErrors, codeMembershipNotFound, "Membership not found")
	case errors.Is(err, store.ErrOrganizationInvitationNotFound):
		return routeError(routeErrors, codeInvitationNotFound, "Invitation not found")
	case errors.Is(err, store.ErrUserNotFound):
		return routeError(routeErrors, codeUserNotFound, "User not found")
	case errors.Is(err, store.ErrInvalidOrganizationName),
		errors.Is(err, organization.ErrInvalidOrganizationRole):
		return routeError(routeErrors, response.CodeInvalidRequest, "Invalid organization request")
	case errors.Is(err, organization.ErrOrganizationOperationForbidden),
		errors.Is(err, organization.ErrOrganizationInviteForbidden):
		return routeError(routeErrors, response.CodeForbidden, "Organization operation forbidden")
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted):
		return routeError(routeErrors, codeInvitationAlreadyAccepted, "Invitation already accepted")
	case errors.Is(err, organization.ErrOrganizationInvitationRevoked):
		return routeError(routeErrors, codeInvitationRevoked, "Invitation already revoked")
	case errors.Is(err, organization.ErrOrganizationInvitationExpired):
		return routeError(routeErrors, codeInvitationExpired, "Invitation expired")
	default:
		return routeError(routeErrors, response.CodeInternalError, "Internal server error")
	}
}

func routeError(routeErrors map[response.ErrorCode]response.ErrorSpec, code response.ErrorCode, message string) contract.Response {
	spec := mustRouteError(routeErrors, code)
	return contract.ErrorJSON(spec.Status, spec.Code, message)
}

func toContractOrganization(org domain.Organization) contract.Organization {
	return contract.Organization{
		Id:              org.ID,
		CreatedAt:       org.CreatedAt,
		UpdatedAt:       org.UpdatedAt,
		Name:            org.Name,
		Kind:            contract.OrganizationKind(org.Kind),
		CreatedByUserId: org.CreatedByUserID,
	}
}

func toContractMembership(membership domain.OrganizationMembership) contract.Membership {
	return contract.Membership{
		OrganizationId: membership.OrganizationID,
		UserId:         membership.UserID,
		Role:           contract.OrganizationRole(membership.Role),
		CreatedAt:      membership.CreatedAt,
		UpdatedAt:      membership.UpdatedAt,
	}
}

func toContractOrganizationMember(member domain.OrganizationMember) contract.OrganizationMember {
	return contract.OrganizationMember{
		OrganizationId: member.Membership.OrganizationID,
		UserId:         member.User.ID,
		Email:          openapi_types.Email(member.User.Email),
		Username:       member.User.Username,
		Role:           contract.OrganizationRole(member.Membership.Role),
		CreatedAt:      member.Membership.CreatedAt,
		UpdatedAt:      member.Membership.UpdatedAt,
		Disabled:       member.User.DisabledAt != nil,
	}
}

func toContractInvitation(invitation domain.OrganizationInvitation, inviteURL string, now time.Time) contract.OrganizationInvitation {
	var url *string
	if inviteURL != "" {
		url = &inviteURL
	}
	return contract.OrganizationInvitation{
		Id:             invitation.ID,
		OrganizationId: invitation.OrganizationID,
		Email:          openapi_types.Email(invitation.Email),
		Role:           contract.OrganizationRole(invitation.Role),
		Status:         contract.OrganizationInvitationStatus(invitation.Status(now)),
		ExpiresAt:      invitation.ExpiresAt.UTC(),
		InviteUrl:      url,
	}
}
