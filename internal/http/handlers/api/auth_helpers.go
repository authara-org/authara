package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func authSignupErrorCode(err error) response.ErrorCode {
	switch {
	case errors.Is(err, auth.ErrEmailNotAllowed):
		return response.CodeForbidden
	case errors.Is(err, auth.ErrUserAlreadyExists),
		errors.Is(err, auth.ErrInvalidUsername),
		errors.Is(err, auth.ErrUnsupportedProvider),
		errors.Is(err, store.ErrOrganizationInvitationNotFound),
		errors.Is(err, organization.ErrInvalidOrganizationInvitationToken),
		errors.Is(err, organization.ErrOrganizationInvitationExpired),
		errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted),
		errors.Is(err, organization.ErrOrganizationInvitationRevoked),
		errors.Is(err, organization.ErrOrganizationInviteEmailMismatch),
		errors.Is(err, organization.ErrOrganizationInviteForbidden),
		errors.Is(err, organization.ErrOrganizationSingleMembershipConflict):
		return response.CodeInvalidRequest
	default:
		return response.CodeInternalError
	}
}

func authSignupErrorMessage(err error, code response.ErrorCode) string {
	if code == response.CodeInternalError {
		return "Signup error."
	}
	switch {
	case errors.Is(err, organization.ErrOrganizationInviteEmailMismatch):
		return "Invitation code does not match this email."
	case errors.Is(err, store.ErrOrganizationInvitationNotFound),
		errors.Is(err, organization.ErrInvalidOrganizationInvitationToken):
		return "Invalid invitation code."
	case errors.Is(err, organization.ErrOrganizationInvitationExpired):
		return "This invitation has expired."
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted):
		return "This invitation has already been accepted."
	case errors.Is(err, organization.ErrOrganizationInvitationRevoked):
		return "This invitation has been revoked."
	default:
		return "Could not create account. Please check your details."
	}
}

func (h *APIHandler) invitationIDForSignupCode(ctx context.Context, email, invitationCode string) (*uuid.UUID, error) {
	if invitationCode == "" {
		return nil, nil
	}
	if h.Organizations == nil {
		return nil, organization.ErrOrganizationInviteForbidden
	}
	preview, err := h.Organizations.InvitationByToken(ctx, invitationCode)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(strings.TrimSpace(preview.Invitation.Email)) != email {
		return nil, organization.ErrOrganizationInviteEmailMismatch
	}
	switch preview.Invitation.Status(time.Now().UTC()) {
	case domain.OrganizationInvitationStatusPending:
	case domain.OrganizationInvitationStatusAccepted:
		return nil, organization.ErrOrganizationInvitationAlreadyAccepted
	case domain.OrganizationInvitationStatusRevoked:
		return nil, organization.ErrOrganizationInvitationRevoked
	case domain.OrganizationInvitationStatusExpired:
		return nil, organization.ErrOrganizationInvitationExpired
	default:
		return nil, organization.ErrOrganizationInviteForbidden
	}
	id := preview.Invitation.ID
	return &id, nil
}

func authLoginErrorCode(err error) response.ErrorCode {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials),
		errors.Is(err, auth.ErrEmailNotAllowed),
		errors.Is(err, store.ErrUserNotFound),
		errors.Is(err, store.ErrorAuthProviderNotFound):
		return response.CodeUnauthorized
	default:
		return response.CodeInternalError
	}
}

func sessionErrorCode(err error) response.ErrorCode {
	if err == nil {
		return ""
	}
	if errors.Is(err, session.ErrForbidden) ||
		errors.Is(err, session.ErrUserDisabled) ||
		errors.Is(err, session.ErrUserNotAllowed) {
		return response.CodeForbidden
	}
	return response.CodeInternalError
}
