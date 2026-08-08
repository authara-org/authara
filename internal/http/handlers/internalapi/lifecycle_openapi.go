package internalapi

import (
	"context"
	"errors"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/store"
)

func (h *Handler) DeleteInternalOrganization(ctx context.Context, request contract.DeleteInternalOrganizationRequestObject) (contract.DeleteInternalOrganizationResponseObject, error) {
	if request.Body == nil {
		return deleteInternalOrganizationError(response.CodeInvalidRequest, "Invalid request body"), nil
	}
	err := h.Organizations.DeleteOrganization(ctx, organization.DeleteOrganizationInput{
		OrganizationID: request.OrganizationID,
		ActorUserID:    request.Body.ActorUserId,
	})
	if err != nil {
		code, message := organizationLifecycleError(err)
		return deleteInternalOrganizationError(code, message), nil
	}
	return contract.DeleteInternalOrganization204Response{}, nil
}

func (h *Handler) RemoveInternalOrganizationMember(ctx context.Context, request contract.RemoveInternalOrganizationMemberRequestObject) (contract.RemoveInternalOrganizationMemberResponseObject, error) {
	if request.Body == nil {
		return removeInternalOrganizationMemberError(response.CodeInvalidRequest, "Invalid request body"), nil
	}
	err := h.Organizations.RemoveOrganizationMember(ctx, organization.RemoveOrganizationMemberInput{
		OrganizationID: request.OrganizationID,
		UserID:         request.UserID,
		ActorUserID:    request.Body.ActorUserId,
	})
	if err != nil {
		code, message := organizationLifecycleError(err)
		return removeInternalOrganizationMemberError(code, message), nil
	}
	return contract.RemoveInternalOrganizationMember204Response{}, nil
}

func (h *Handler) TransferInternalOrganizationOwnership(ctx context.Context, request contract.TransferInternalOrganizationOwnershipRequestObject) (contract.TransferInternalOrganizationOwnershipResponseObject, error) {
	if request.Body == nil {
		return transferInternalOrganizationOwnershipError(response.CodeInvalidRequest, "Invalid request body"), nil
	}
	err := h.Organizations.TransferOrganizationOwnership(ctx, organization.TransferOrganizationOwnershipInput{
		OrganizationID: request.OrganizationID,
		ActorUserID:    request.Body.ActorUserId,
		NewOwnerUserID: request.Body.NewOwnerUserId,
	})
	if err != nil {
		code, message := organizationLifecycleError(err)
		return transferInternalOrganizationOwnershipError(code, message), nil
	}
	return contract.TransferInternalOrganizationOwnership204Response{}, nil
}

func (h *Handler) DeleteInternalUser(ctx context.Context, request contract.DeleteInternalUserRequestObject) (contract.DeleteInternalUserResponseObject, error) {
	err := h.Auth.DeleteUser(ctx, request.UserID)
	if err != nil {
		code, message := userLifecycleError(err)
		return deleteInternalUserError(code, message), nil
	}
	return contract.DeleteInternalUser204Response{}, nil
}

func organizationLifecycleError(err error) (response.ErrorCode, string) {
	switch {
	case errors.Is(err, organization.ErrInvalidOrganizationOwnershipTransfer):
		return response.CodeInvalidRequest, "New owner must differ from current owner"
	case errors.Is(err, store.ErrOrganizationNotFound):
		return codeOrganizationNotFound, "Organization not found"
	case errors.Is(err, store.ErrOrganizationMembershipNotFound):
		return codeMembershipNotFound, "Membership not found"
	case errors.Is(err, organization.ErrOrganizationActorNotMember):
		return codeActorNotMember, "Actor is not a member of this organization"
	case errors.Is(err, organization.ErrOrganizationActorNotAllowed):
		return codeActorNotAllowed, "Actor is not allowed to perform this operation"
	case errors.Is(err, organization.ErrLastOrganizationOwner):
		return codeLastOrganizationOwner, "Transfer ownership before leaving the organization"
	case errors.Is(err, organization.ErrLastOrganizationMember):
		return codeLastOrganizationMember, "The last member must delete the organization"
	case errors.Is(err, organization.ErrOrganizationHasOtherMembers):
		return codeOrganizationHasOtherMembers, "A single-mode organization with other members cannot be deleted"
	case errors.Is(err, organization.ErrPersonalOrganizationImmutable):
		return codePersonalOrganizationImmutable, "Personal organization creators cannot leave and personal organizations cannot be deleted"
	default:
		return response.CodeInternalError, "Internal server error"
	}
}

func userLifecycleError(err error) (response.ErrorCode, string) {
	switch {
	case errors.Is(err, store.ErrUserNotFound):
		return codeUserNotFound, "User not found"
	case errors.Is(err, auth.ErrCannotDeleteLastAdmin):
		return codeLastActiveAdmin, "The last active administrator cannot be deleted"
	case errors.Is(err, organization.ErrLastOrganizationOwner):
		return codeLastOrganizationOwner, "Transfer ownership before deleting this user"
	case errors.Is(err, organization.ErrLastOrganizationMember):
		return codeLastOrganizationMember, "Delete the user's organization before deleting this user"
	default:
		return response.CodeInternalError, "Internal server error"
	}
}
