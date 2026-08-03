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

func (h *Handler) ListPublicOrganizationInvitations(ctx context.Context, request contract.ListPublicOrganizationInvitationsRequestObject) (contract.ListPublicOrganizationInvitationsResponseObject, error) {
	return h.listOrganizationInvitations(ctx, request.OrganizationID), nil
}

func (h *Handler) GetPublicOrganizationInvitation(ctx context.Context, request contract.GetPublicOrganizationInvitationRequestObject) (contract.GetPublicOrganizationInvitationResponseObject, error) {
	return h.getOrganizationInvitation(ctx, request.OrganizationID, request.InvitationID), nil
}

func (h *Handler) RevokePublicOrganizationInvitation(ctx context.Context, request contract.RevokePublicOrganizationInvitationRequestObject) (contract.RevokePublicOrganizationInvitationResponseObject, error) {
	actorUserID, ok := httpctx.UserID(ctx)
	if !ok {
		return revokePublicOrganizationInvitationError(response.CodeUnauthorized, "Unauthorized"), nil
	}
	return h.revokeOrganizationInvitation(ctx, request.OrganizationID, request.InvitationID, &actorUserID), nil
}

func (h *Handler) CreateInternalOrganizationInvitation(ctx context.Context, request contract.CreateInternalOrganizationInvitationRequestObject) (contract.CreateInternalOrganizationInvitationResponseObject, error) {
	if request.Body == nil {
		return createInternalOrganizationInvitationError(response.CodeInvalidRequest, "Invalid request body"), nil
	}
	var role domain.OrganizationRole
	if request.Body.Role != nil {
		role = domain.OrganizationRole(*request.Body.Role)
	}
	var metadata map[string]any
	if request.Body.Metadata != nil {
		metadata = *request.Body.Metadata
	}
	return h.createOrganizationInvitation(
		ctx,
		request.OrganizationID,
		request.Body.ActorUserId,
		string(request.Body.Email),
		role,
		metadata,
	), nil
}

func (h *Handler) ResendInternalOrganizationInvitation(ctx context.Context, request contract.ResendInternalOrganizationInvitationRequestObject) (contract.ResendInternalOrganizationInvitationResponseObject, error) {
	return h.resendOrganizationInvitation(ctx, request.OrganizationID, request.InvitationID), nil
}

func (h *Handler) listOrganizationInvitations(ctx context.Context, organizationID openapi_types.UUID) contract.ListPublicOrganizationInvitationsResponseObject {
	if _, code, message, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, true); !ok {
		return listPublicOrganizationInvitationsError(code, message)
	}
	now := time.Now().UTC()
	invitations, err := h.Organizations.ListInvitations(ctx, organizationID)
	if err != nil {
		code, message := organizationError(err)
		return listPublicOrganizationInvitationsError(code, message)
	}
	outInvitations := make([]contract.OrganizationInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		outInvitations = append(outInvitations, toContractInvitation(invitation, "", now))
	}
	return contract.ListPublicOrganizationInvitations200JSONResponse(contract.OrganizationInvitations{Invitations: outInvitations})
}

func (h *Handler) getOrganizationInvitation(ctx context.Context, organizationID, invitationID openapi_types.UUID) contract.GetPublicOrganizationInvitationResponseObject {
	if _, code, message, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, true); !ok {
		return getPublicOrganizationInvitationError(code, message)
	}
	preview, err := h.Organizations.InvitationByOrganizationAndID(ctx, organizationID, invitationID)
	if err != nil {
		code, message := organizationError(err)
		return getPublicOrganizationInvitationError(code, message)
	}
	return contract.GetPublicOrganizationInvitation200JSONResponse(contract.OrganizationInvitationEnvelope{
		Invitation: toContractInvitation(preview.Invitation, "", time.Now().UTC()),
	})
}

func (h *Handler) createOrganizationInvitation(
	ctx context.Context,
	organizationID, actorUserID openapi_types.UUID,
	email string,
	role domain.OrganizationRole,
	metadata map[string]any,
) contract.CreateInternalOrganizationInvitationResponseObject {
	now := time.Now().UTC()
	out, err := h.Organizations.CreateInvitation(ctx, organization.CreateInvitationInput{
		OrganizationID: organizationID,
		ActorUserID:    actorUserID,
		Email:          email,
		Role:           role,
		Metadata:       metadata,
		Now:            now,
	})
	if err != nil {
		code, message := createInvitationError(err)
		return createInternalOrganizationInvitationError(code, message)
	}
	return contract.CreateInternalOrganizationInvitation201JSONResponse(contract.OrganizationInvitationEnvelope{
		Invitation: toContractInvitation(out.Invitation, out.InviteURL, now),
	})
}

func (h *Handler) resendOrganizationInvitation(ctx context.Context, organizationID, invitationID openapi_types.UUID) contract.ResendInternalOrganizationInvitationResponseObject {
	if _, code, message, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, true); !ok {
		return resendInternalOrganizationInvitationError(code, message)
	}
	now := time.Now().UTC()
	out, err := h.Organizations.ResendInvitation(ctx, organization.ResendInvitationInput{
		OrganizationID: organizationID,
		InvitationID:   invitationID,
		Now:            now,
	})
	if err != nil {
		code, message := resendInvitationError(err)
		return resendInternalOrganizationInvitationError(code, message)
	}
	return contract.ResendInternalOrganizationInvitation201JSONResponse(contract.OrganizationInvitationEnvelope{
		Invitation: toContractInvitation(out.Invitation, out.InviteURL, now),
	})
}

func (h *Handler) revokeOrganizationInvitation(ctx context.Context, organizationID, invitationID openapi_types.UUID, revokedBy *openapi_types.UUID) contract.RevokePublicOrganizationInvitationResponseObject {
	if _, code, message, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, true); !ok {
		return revokePublicOrganizationInvitationError(code, message)
	}
	now := time.Now().UTC()
	invitation, err := h.Organizations.RevokeInvitation(ctx, organization.RevokeInvitationInput{
		OrganizationID:  organizationID,
		InvitationID:    invitationID,
		RevokedByUserID: revokedBy,
		Now:             now,
	})
	if err != nil {
		code, message := organizationError(err)
		return revokePublicOrganizationInvitationError(code, message)
	}
	return contract.RevokePublicOrganizationInvitation200JSONResponse(contract.OrganizationInvitationEnvelope{
		Invitation: toContractInvitation(invitation, "", now),
	})
}

func createInvitationError(err error) (response.ErrorCode, string) {
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		return codeOrganizationNotFound, "Organization not found"
	case errors.Is(err, organization.ErrOrganizationActorNotMember):
		return codeActorNotMember, "Actor is not a member of this organization"
	case errors.Is(err, organization.ErrOrganizationInviteForbidden):
		return codeActorNotAllowed, "Actor is not allowed to invite members"
	case errors.Is(err, organization.ErrOrganizationMemberAlreadyExists):
		return codeAlreadyMember, "User is already a member"
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyPending):
		return codeInvitationAlreadyPending, "Invitation already pending"
	case errors.Is(err, organization.ErrInvalidOrganizationInvitationEmail),
		errors.Is(err, organization.ErrInvalidOrganizationInvitationMetadata),
		errors.Is(err, organization.ErrInvalidOrganizationRole):
		return response.CodeInvalidRequest, "Invalid invitation request"
	default:
		return response.CodeInternalError, "Internal server error"
	}
}

func resendInvitationError(err error) (response.ErrorCode, string) {
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		return codeOrganizationNotFound, "Organization not found"
	case errors.Is(err, store.ErrOrganizationInvitationNotFound):
		return codeInvitationNotFound, "Invitation not found"
	case errors.Is(err, organization.ErrOrganizationInviteForbidden):
		return response.CodeForbidden, "Organization operation forbidden"
	case errors.Is(err, organization.ErrOrganizationMemberAlreadyExists):
		return codeAlreadyMember, "User is already a member"
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyPending):
		return codeInvitationAlreadyPending, "Invitation already pending"
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted):
		return codeInvitationAlreadyAccepted, "Invitation already accepted"
	case errors.Is(err, organization.ErrOrganizationInvitationRevoked):
		return codeInvitationRevoked, "Invitation already revoked"
	default:
		return response.CodeInternalError, "Internal server error"
	}
}
