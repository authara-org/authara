package internalapi

import (
	"context"
	"encoding/json"
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
	managerOnly bool,
) (openapi_types.UUID, response.ErrorCode, string, bool) {
	userID, publicRequest := httpctx.UserID(ctx)
	if !publicRequest {
		return openapi_types.UUID{}, "", "", true
	}
	currentOrganizationID, ok := httpctx.OrganizationID(ctx)
	if !ok || currentOrganizationID != organizationID {
		return openapi_types.UUID{}, response.CodeForbidden, "Organization operation forbidden", false
	}
	membership, err := h.Organizations.RequireMembership(ctx, userID, organizationID)
	if errors.Is(err, store.ErrOrganizationMembershipNotFound) {
		return openapi_types.UUID{}, response.CodeForbidden, "Organization operation forbidden", false
	}
	if err != nil {
		return openapi_types.UUID{}, response.CodeInternalError, "Internal server error", false
	}
	if managerOnly && membership.Role != domain.OrganizationRoleOwner && membership.Role != domain.OrganizationRoleAdmin {
		return openapi_types.UUID{}, response.CodeForbidden, "Organization operation forbidden", false
	}
	return userID, "", "", true
}

func organizationError(err error) (response.ErrorCode, string) {
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		return codeOrganizationNotFound, "Organization not found"
	case errors.Is(err, store.ErrOrganizationMembershipNotFound):
		return codeMembershipNotFound, "Membership not found"
	case errors.Is(err, store.ErrOrganizationInvitationNotFound):
		return codeInvitationNotFound, "Invitation not found"
	case errors.Is(err, store.ErrUserNotFound):
		return codeUserNotFound, "User not found"
	case errors.Is(err, store.ErrInvalidOrganizationName),
		errors.Is(err, organization.ErrInvalidOrganizationRole):
		return response.CodeInvalidRequest, "Invalid organization request"
	case errors.Is(err, organization.ErrOrganizationOperationForbidden),
		errors.Is(err, organization.ErrOrganizationInviteForbidden):
		return response.CodeForbidden, "Organization operation forbidden"
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted):
		return codeInvitationAlreadyAccepted, "Invitation already accepted"
	case errors.Is(err, organization.ErrOrganizationInvitationRevoked):
		return codeInvitationRevoked, "Invitation already revoked"
	case errors.Is(err, organization.ErrOrganizationInvitationExpired):
		return codeInvitationExpired, "Invitation expired"
	default:
		return response.CodeInternalError, "Internal server error"
	}
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
	metadata := map[string]any{}
	if len(invitation.Metadata) > 0 {
		_ = json.Unmarshal(invitation.Metadata, &metadata)
	}
	return contract.OrganizationInvitation{
		Id:             invitation.ID,
		OrganizationId: invitation.OrganizationID,
		Email:          openapi_types.Email(invitation.Email),
		Role:           contract.OrganizationRole(invitation.Role),
		Metadata:       metadata,
		Status:         contract.OrganizationInvitationStatus(invitation.Status(now)),
		ExpiresAt:      invitation.ExpiresAt.UTC(),
		InviteUrl:      url,
	}
}
