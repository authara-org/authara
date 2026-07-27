package internalapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/store"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *Handler) ListPublicOrganizationInvitations(ctx context.Context, request contract.ListPublicOrganizationInvitationsRequestObject) (contract.ListPublicOrganizationInvitationsResponseObject, error) {
	return h.listOrganizationInvitations(ctx, request.OrganizationID, ListPublicOrganizationInvitationsErrors), nil
}

func (h *Handler) GetPublicOrganizationInvitation(ctx context.Context, request contract.GetPublicOrganizationInvitationRequestObject) (contract.GetPublicOrganizationInvitationResponseObject, error) {
	return h.getOrganizationInvitation(ctx, request.OrganizationID, request.InvitationID, GetPublicOrganizationInvitationErrors), nil
}

func (h *Handler) RevokePublicOrganizationInvitation(ctx context.Context, request contract.RevokePublicOrganizationInvitationRequestObject) (contract.RevokePublicOrganizationInvitationResponseObject, error) {
	actorUserID, ok := httpctx.UserID(ctx)
	if !ok {
		return routeError(RevokePublicOrganizationInvitationErrors, response.CodeUnauthorized, "Unauthorized"), nil
	}
	return h.revokeOrganizationInvitation(ctx, request.OrganizationID, request.InvitationID, &actorUserID, RevokePublicOrganizationInvitationErrors), nil
}

func (h *Handler) CreateInternalOrganizationInvitation(ctx context.Context, request contract.CreateInternalOrganizationInvitationRequestObject) (contract.CreateInternalOrganizationInvitationResponseObject, error) {
	if request.Body == nil {
		return routeError(CreateInternalOrganizationInvitationErrors, response.CodeInvalidRequest, "Invalid request body"), nil
	}
	return h.createOrganizationInvitation(ctx, request.OrganizationID, request.Body.ActorUserId, string(request.Body.Email), CreateInternalOrganizationInvitationErrors), nil
}

func (h *Handler) ResendInternalOrganizationInvitation(ctx context.Context, request contract.ResendInternalOrganizationInvitationRequestObject) (contract.ResendInternalOrganizationInvitationResponseObject, error) {
	return h.resendOrganizationInvitation(ctx, request.OrganizationID, request.InvitationID, ResendInternalOrganizationInvitationErrors), nil
}

func (h *Handler) listOrganizationInvitations(ctx context.Context, organizationID openapi_types.UUID, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	if _, out, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, routeErrors, true); !ok {
		return out
	}
	now := time.Now().UTC()
	invitations, err := h.Organizations.ListInvitations(ctx, organizationID)
	if err != nil {
		return organizationError(routeErrors, err)
	}
	outInvitations := make([]contract.OrganizationInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		outInvitations = append(outInvitations, toContractInvitation(invitation, "", now))
	}
	return contract.JSON(http.StatusOK, contract.OrganizationInvitations{Invitations: outInvitations})
}

func (h *Handler) getOrganizationInvitation(ctx context.Context, organizationID, invitationID openapi_types.UUID, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	if _, out, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, routeErrors, true); !ok {
		return out
	}
	preview, err := h.Organizations.InvitationByOrganizationAndID(ctx, organizationID, invitationID)
	if err != nil {
		return organizationError(routeErrors, err)
	}
	return contract.JSON(http.StatusOK, contract.OrganizationInvitationEnvelope{
		Invitation: toContractInvitation(preview.Invitation, "", time.Now().UTC()),
	})
}

func (h *Handler) createOrganizationInvitation(ctx context.Context, organizationID, actorUserID openapi_types.UUID, email string, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	now := time.Now().UTC()
	out, err := h.Organizations.CreateInvitation(ctx, organization.CreateInvitationInput{
		OrganizationID: organizationID,
		ActorUserID:    actorUserID,
		Email:          email,
		Now:            now,
	})
	if err != nil {
		return createInvitationError(routeErrors, err)
	}
	return contract.JSON(http.StatusCreated, contract.OrganizationInvitationEnvelope{
		Invitation: toContractInvitation(out.Invitation, out.InviteURL, now),
	})
}

func (h *Handler) resendOrganizationInvitation(ctx context.Context, organizationID, invitationID openapi_types.UUID, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	if _, out, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, routeErrors, true); !ok {
		return out
	}
	now := time.Now().UTC()
	out, err := h.Organizations.ResendInvitation(ctx, organization.ResendInvitationInput{
		OrganizationID: organizationID,
		InvitationID:   invitationID,
		Now:            now,
	})
	if err != nil {
		return resendInvitationError(routeErrors, err)
	}
	return contract.JSON(http.StatusCreated, contract.OrganizationInvitationEnvelope{
		Invitation: toContractInvitation(out.Invitation, out.InviteURL, now),
	})
}

func (h *Handler) revokeOrganizationInvitation(ctx context.Context, organizationID, invitationID openapi_types.UUID, revokedBy *openapi_types.UUID, routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	if _, out, ok := h.contractAuthorizePublicOrganization(ctx, organizationID, routeErrors, true); !ok {
		return out
	}
	now := time.Now().UTC()
	invitation, err := h.Organizations.RevokeInvitation(ctx, organization.RevokeInvitationInput{
		OrganizationID:  organizationID,
		InvitationID:    invitationID,
		RevokedByUserID: revokedBy,
		Now:             now,
	})
	if err != nil {
		return organizationError(routeErrors, err)
	}
	return contract.JSON(http.StatusOK, contract.OrganizationInvitationEnvelope{
		Invitation: toContractInvitation(invitation, "", now),
	})
}

func createInvitationError(routeErrors map[response.ErrorCode]response.ErrorSpec, err error) contract.Response {
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		return routeError(routeErrors, codeOrganizationNotFound, "Organization not found")
	case errors.Is(err, organization.ErrOrganizationActorNotMember):
		return routeError(routeErrors, codeActorNotMember, "Actor is not a member of this organization")
	case errors.Is(err, organization.ErrOrganizationInviteForbidden):
		return routeError(routeErrors, codeActorNotAllowed, "Actor is not allowed to invite members")
	case errors.Is(err, organization.ErrOrganizationMemberAlreadyExists):
		return routeError(routeErrors, codeAlreadyMember, "User is already a member")
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyPending):
		return routeError(routeErrors, codeInvitationAlreadyPending, "Invitation already pending")
	case errors.Is(err, organization.ErrInvalidOrganizationInvitationEmail):
		return routeError(routeErrors, response.CodeInvalidRequest, "Invalid invitation request")
	default:
		return routeError(routeErrors, response.CodeInternalError, "Internal server error")
	}
}

func resendInvitationError(routeErrors map[response.ErrorCode]response.ErrorSpec, err error) contract.Response {
	switch {
	case errors.Is(err, store.ErrOrganizationNotFound):
		return routeError(routeErrors, codeOrganizationNotFound, "Organization not found")
	case errors.Is(err, store.ErrOrganizationInvitationNotFound):
		return routeError(routeErrors, codeInvitationNotFound, "Invitation not found")
	case errors.Is(err, organization.ErrOrganizationInviteForbidden):
		return routeError(routeErrors, response.CodeForbidden, "Organization operation forbidden")
	case errors.Is(err, organization.ErrOrganizationMemberAlreadyExists):
		return routeError(routeErrors, codeAlreadyMember, "User is already a member")
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyPending):
		return routeError(routeErrors, codeInvitationAlreadyPending, "Invitation already pending")
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted):
		return routeError(routeErrors, codeInvitationAlreadyAccepted, "Invitation already accepted")
	case errors.Is(err, organization.ErrOrganizationInvitationRevoked):
		return routeError(routeErrors, codeInvitationRevoked, "Invitation already revoked")
	default:
		return routeError(routeErrors, response.CodeInternalError, "Internal server error")
	}
}
