package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *APIHandler) PreviewInvitation(ctx context.Context, request contract.PreviewInvitationRequestObject) (contract.PreviewInvitationResponseObject, error) {
	rawToken := strings.TrimSpace(request.Params.Token)
	if rawToken == "" {
		return previewInvitationError(response.CodeInvalidRequest, "Invitation token required."), nil
	}
	preview, err := h.Organizations.InvitationByToken(ctx, rawToken)
	if err != nil {
		code, message := invitationError(err)
		return previewInvitationError(code, message), nil
	}
	return contract.PreviewInvitation200JSONResponse(toContractInvitationPreview(preview, time.Now().UTC())), nil
}

func (h *APIHandler) AcceptInvitation(ctx context.Context, request contract.AcceptInvitationRequestObject) (contract.AcceptInvitationResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return acceptInvitationError(response.CodeUnauthorized, "Unauthorized."), nil
	}
	sessionID, ok := httpctx.SessionID(ctx)
	if !ok {
		return acceptInvitationError(response.CodeUnauthorized, "Unauthorized."), nil
	}
	if request.Body == nil || strings.TrimSpace(request.Body.Token) == "" {
		return acceptInvitationError(response.CodeInvalidRequest, "Invitation token required."), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	now := time.Now().UTC()
	result, err := h.Organizations.AcceptInvitation(ctx, organization.AcceptInvitationInput{
		RawToken: request.Body.Token,
		UserID:   userID,
		Now:      now,
	})
	if err != nil {
		code, message := invitationError(err)
		return acceptInvitationError(code, message), nil
	}
	accessToken, refreshToken, err := h.Session.SwitchSessionOrganization(ctx, userID, sessionID, result.Organization.ID, audience, now)
	if err != nil {
		code, message := invitationSessionError(err)
		return acceptInvitationError(code, message), nil
	}
	header := sessionHeader(h, accessToken, refreshToken)
	return contract.AcceptInvitation200HeadersResponse{
		Header: header,
		Body:   contract.Tokens{AccessToken: accessToken, RefreshToken: refreshToken},
	}, nil
}

func (h *APIHandler) LoginAndAcceptInvitation(ctx context.Context, request contract.LoginAndAcceptInvitationRequestObject) (contract.LoginAndAcceptInvitationResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return loginAndAcceptInvitationError(response.CodeInternalError, "API contract error."), nil
	}
	if request.Body == nil || strings.TrimSpace(request.Body.Token) == "" || request.Body.Password == "" {
		return loginAndAcceptInvitationError(response.CodeInvalidRequest, "Invitation token and password required."), nil
	}
	preview, code, message, ok := h.pendingInvitation(ctx, request.Body.Token)
	if !ok {
		return loginAndAcceptInvitationError(code, message), nil
	}
	allowed, err := h.Limiter.AllowLoginAttempt(ctx, httputil.ClientIP(r), preview.Invitation.Email)
	if err != nil || !allowed {
		return loginAndAcceptInvitationError(response.CodeRateLimited, "Too many attempts. Please try again later."), nil
	}
	user, err := h.Auth.Login(ctx, auth.LoginInput{
		Provider:        domain.ProviderPassword,
		Email:           preview.Invitation.Email,
		Password:        request.Body.Password,
		InvitationToken: request.Body.Token,
	})
	if err != nil {
		return loginAndAcceptInvitationError(response.CodeUnauthorized, "Invalid email or password."), nil
	}
	result, err := h.Organizations.AcceptInvitation(ctx, organization.AcceptInvitationInput{
		RawToken: request.Body.Token,
		UserID:   user.ID,
		Now:      time.Now().UTC(),
	})
	if err != nil {
		code, message := invitationError(err)
		return loginAndAcceptInvitationError(code, message), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	body, header, code, message, ok := h.contractInvitationSession(ctx, r, user, result.Organization.ID, audience)
	if !ok {
		return loginAndAcceptInvitationError(code, message), nil
	}
	return contract.LoginAndAcceptInvitation200HeadersResponse{Header: header, Body: body}, nil
}

func (h *APIHandler) AuthenticateAndAcceptInvitationWithGoogle(ctx context.Context, request contract.AuthenticateAndAcceptInvitationWithGoogleRequestObject) (contract.AuthenticateAndAcceptInvitationWithGoogleResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return authenticateAndAcceptInvitationWithGoogleError(response.CodeInternalError, "API contract error."), nil
	}
	if request.Body == nil || strings.TrimSpace(request.Body.Token) == "" {
		return authenticateAndAcceptInvitationWithGoogleError(response.CodeInvalidRequest, "Invitation token required."), nil
	}
	identity, header, code, message, ok := h.verifyGoogleCredential(ctx, r, request.Body.Credential, request.Body.Nonce)
	if !ok {
		return authenticateAndAcceptInvitationWithGoogleError(code, message), nil
	}
	preview, code, message, ok := h.pendingInvitation(ctx, request.Body.Token)
	if !ok {
		return authenticateAndAcceptInvitationWithGoogleError(code, message), nil
	}
	if normalizeEmail(identity.Email) != normalizeEmail(preview.Invitation.Email) {
		return authenticateAndAcceptInvitationWithGoogleError(codeInvitationEmailMismatch, "This invitation is for a different account."), nil
	}
	exists, err := h.Auth.UserExistsByEmail(ctx, preview.Invitation.Email)
	if err != nil {
		return authenticateAndAcceptInvitationWithGoogleError(response.CodeInternalError, "Invitation login error."), nil
	}
	if (request.Body.Flow == contract.Signup && exists) || (request.Body.Flow == contract.Login && !exists) {
		return authenticateAndAcceptInvitationWithGoogleError(codeInvitationFlowMismatch, "Invitation signup or login flow does not match the account."), nil
	}
	user, err := h.Auth.Login(ctx, auth.LoginInput{
		Provider:        domain.ProviderGoogle,
		Email:           preview.Invitation.Email,
		OAuthID:         identity.OAuthID,
		InvitationToken: request.Body.Token,
	})
	if errors.Is(err, auth.ErrAccountExistsMustLink) {
		link, code, message, ok := h.startAccountRecoveryLink(ctx, identity)
		if !ok {
			return authenticateAndAcceptInvitationWithGoogleError(code, message), nil
		}
		return contract.AuthenticateAndAcceptInvitationWithGoogle200HeadersResponse{
			Header: header,
			Body: contract.InvitationGoogleResult{
				Status:   contract.ProofRequired,
				Recovery: &link,
			},
		}, nil
	}
	if err != nil {
		code, message := invitationOrGoogleError(err)
		return authenticateAndAcceptInvitationWithGoogleError(code, message), nil
	}
	result, err := h.Organizations.AcceptInvitation(ctx, organization.AcceptInvitationInput{
		RawToken: request.Body.Token,
		UserID:   user.ID,
		Now:      time.Now().UTC(),
	})
	if err != nil {
		code, message := invitationError(err)
		return authenticateAndAcceptInvitationWithGoogleError(code, message), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	body, sessionHeaders, code, message, ok := h.contractInvitationSession(ctx, r, user, result.Organization.ID, audience)
	if !ok {
		return authenticateAndAcceptInvitationWithGoogleError(code, message), nil
	}
	copyHeaders(sessionHeaders, header)
	return contract.AuthenticateAndAcceptInvitationWithGoogle200HeadersResponse{
		Header: sessionHeaders,
		Body: contract.InvitationGoogleResult{
			Status:  contract.Authenticated,
			Session: &body,
		},
	}, nil
}

func (h *APIHandler) StartGoogleAccountRecoveryLink(ctx context.Context, request contract.StartGoogleAccountRecoveryLinkRequestObject) (contract.StartGoogleAccountRecoveryLinkResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return startGoogleAccountRecoveryLinkError(response.CodeInternalError, "API contract error."), nil
	}
	if request.Body == nil {
		return startGoogleAccountRecoveryLinkError(response.CodeInvalidRequest, "Invalid JSON body."), nil
	}
	identity, header, code, message, ok := h.verifyGoogleCredential(ctx, r, request.Body.Credential, request.Body.Nonce)
	if !ok {
		return startGoogleAccountRecoveryLinkError(code, message), nil
	}
	link, code, message, ok := h.startAccountRecoveryLink(ctx, identity)
	if !ok {
		return startGoogleAccountRecoveryLinkError(code, message), nil
	}
	return contract.StartGoogleAccountRecoveryLink202HeadersResponse{Header: header, Body: link}, nil
}

func (h *APIHandler) CompleteAccountRecoveryLinkWithPassword(ctx context.Context, request contract.CompleteAccountRecoveryLinkWithPasswordRequestObject) (contract.CompleteAccountRecoveryLinkWithPasswordResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return completeAccountRecoveryLinkWithPasswordError(response.CodeInternalError, "API contract error."), nil
	}
	if request.Body == nil || request.Body.Password == "" {
		return completeAccountRecoveryLinkWithPasswordError(response.CodeInvalidRequest, "Password required."), nil
	}
	link, user, code, message, ok := h.recoveryLinkAndUser(ctx, request.LinkID)
	if !ok {
		return completeAccountRecoveryLinkWithPasswordError(code, message), nil
	}
	invitationToken := optionalString(request.Body.InvitationToken)
	if invitationToken != "" {
		if code, message, ok := h.validateRecoveryInvitation(ctx, invitationToken, user.Email); !ok {
			return completeAccountRecoveryLinkWithPasswordError(code, message), nil
		}
	}
	email := user.Email
	if link.ProviderEmail != nil {
		email = *link.ProviderEmail
	}
	allowed, err := h.Limiter.AllowLoginAttempt(ctx, httputil.ClientIP(r), email)
	if err != nil || !allowed {
		return completeAccountRecoveryLinkWithPasswordError(response.CodeRateLimited, "Too many attempts. Please try again later."), nil
	}
	user, err = h.Auth.CompleteAccountRecoveryProviderLinkWithPassword(ctx, request.LinkID, request.Body.Password, time.Now().UTC())
	if err != nil {
		code, message := recoveryError(err)
		return completeAccountRecoveryLinkWithPasswordError(code, message), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	body, header, code, message, ok := h.finishRecoverySession(ctx, r, user, invitationToken, audience)
	if !ok {
		return completeAccountRecoveryLinkWithPasswordError(code, message), nil
	}
	return contract.CompleteAccountRecoveryLinkWithPassword200HeadersResponse{Header: header, Body: body}, nil
}

func (h *APIHandler) CompleteAccountRecoveryLinkWithGoogle(ctx context.Context, request contract.CompleteAccountRecoveryLinkWithGoogleRequestObject) (contract.CompleteAccountRecoveryLinkWithGoogleResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return completeAccountRecoveryLinkWithGoogleError(response.CodeInternalError, "API contract error."), nil
	}
	if request.Body == nil {
		return completeAccountRecoveryLinkWithGoogleError(response.CodeInvalidRequest, "Invalid JSON body."), nil
	}
	_, targetUser, code, message, ok := h.recoveryLinkAndUser(ctx, request.LinkID)
	if !ok {
		return completeAccountRecoveryLinkWithGoogleError(code, message), nil
	}
	invitationToken := optionalString(request.Body.InvitationToken)
	if invitationToken != "" {
		if code, message, ok := h.validateRecoveryInvitation(ctx, invitationToken, targetUser.Email); !ok {
			return completeAccountRecoveryLinkWithGoogleError(code, message), nil
		}
	}
	identity, nonceHeader, code, message, ok := h.verifyGoogleCredential(ctx, r, request.Body.Credential, request.Body.Nonce)
	if !ok {
		return completeAccountRecoveryLinkWithGoogleError(code, message), nil
	}
	user, err := h.Auth.CompleteAccountRecoveryProviderLinkWithProviderProof(ctx, request.LinkID, domain.ProviderGoogle, identity.OAuthID, time.Now().UTC())
	if err != nil {
		code, message := recoveryError(err)
		return completeAccountRecoveryLinkWithGoogleError(code, message), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	body, header, code, message, ok := h.finishRecoverySession(ctx, r, user, invitationToken, audience)
	if !ok {
		return completeAccountRecoveryLinkWithGoogleError(code, message), nil
	}
	copyHeaders(header, nonceHeader)
	return contract.CompleteAccountRecoveryLinkWithGoogle200HeadersResponse{Header: header, Body: body}, nil
}

func (h *APIHandler) pendingInvitation(ctx context.Context, rawToken string) (organization.InvitationPreview, response.ErrorCode, string, bool) {
	preview, err := h.Organizations.InvitationByToken(ctx, strings.TrimSpace(rawToken))
	if err != nil {
		code, message := invitationError(err)
		return organization.InvitationPreview{}, code, message, false
	}
	switch preview.Invitation.Status(time.Now().UTC()) {
	case domain.OrganizationInvitationStatusPending:
		return preview, "", "", true
	case domain.OrganizationInvitationStatusAccepted:
		return organization.InvitationPreview{}, codeInvitationAlreadyAccepted, "Invitation already accepted.", false
	case domain.OrganizationInvitationStatusRevoked:
		return organization.InvitationPreview{}, codeInvitationRevoked, "Invitation revoked.", false
	default:
		return organization.InvitationPreview{}, codeInvitationExpired, "Invitation expired.", false
	}
}

func (h *APIHandler) startAccountRecoveryLink(ctx context.Context, identity *google.Identity) (contract.AccountRecoveryLink, response.ErrorCode, string, bool) {
	link, err := h.Auth.StartAccountRecoveryProviderLink(ctx, auth.OAuthIdentityInput{
		Provider:              domain.ProviderGoogle,
		Email:                 identity.Email,
		ProviderUserID:        identity.OAuthID,
		ProviderEmailVerified: identity.EmailVerified,
	}, time.Now().UTC())
	if err != nil {
		code, message := recoveryError(err)
		return contract.AccountRecoveryLink{}, code, message, false
	}
	providers, err := h.Auth.ListUserAuthProviders(ctx, link.UserID)
	if err != nil {
		return contract.AccountRecoveryLink{}, response.CodeInternalError, "Account recovery error.", false
	}
	proofs := make([]contract.AccountRecoveryLinkProofMethods, 0, len(providers))
	for _, provider := range providers {
		switch provider.Provider {
		case domain.ProviderPassword:
			proofs = append(proofs, contract.AccountRecoveryLinkProofMethodsPassword)
		case domain.ProviderGoogle:
			if h.Google != nil {
				proofs = append(proofs, contract.AccountRecoveryLinkProofMethodsGoogle)
			}
		}
	}
	return contract.AccountRecoveryLink{LinkId: link.ID, ProofMethods: proofs}, "", "", true
}

func (h *APIHandler) recoveryLinkAndUser(ctx context.Context, linkID contract.ProviderLinkID) (domain.PendingProviderLink, domain.User, response.ErrorCode, string, bool) {
	link, err := h.Auth.GetPendingProviderLink(ctx, linkID)
	if err != nil {
		code, message := recoveryError(err)
		return domain.PendingProviderLink{}, domain.User{}, code, message, false
	}
	if link.ConsumedAt != nil || !link.ExpiresAt.After(time.Now().UTC()) {
		return domain.PendingProviderLink{}, domain.User{}, codeProviderLinkExpired, "Provider link expired.", false
	}
	user, err := h.Auth.GetUser(ctx, link.UserID)
	if err != nil {
		return domain.PendingProviderLink{}, domain.User{}, response.CodeInternalError, "Account recovery error.", false
	}
	return link, user, "", "", true
}

func (h *APIHandler) validateRecoveryInvitation(ctx context.Context, rawToken string, email string) (response.ErrorCode, string, bool) {
	preview, code, message, ok := h.pendingInvitation(ctx, rawToken)
	if !ok {
		return code, message, false
	}
	if normalizeEmail(preview.Invitation.Email) != normalizeEmail(email) {
		return codeInvitationEmailMismatch, "This invitation is for a different account.", false
	}
	return "", "", true
}

func (h *APIHandler) finishRecoverySession(ctx context.Context, r *http.Request, user domain.User, rawToken string, audience token.Audience) (contract.AuthSession, http.Header, response.ErrorCode, string, bool) {
	if rawToken == "" {
		return h.contractSession(ctx, r, user, audience)
	}
	result, err := h.Organizations.AcceptInvitation(ctx, organization.AcceptInvitationInput{
		RawToken: rawToken,
		UserID:   user.ID,
		Now:      time.Now().UTC(),
	})
	if err != nil {
		code, message := invitationError(err)
		return contract.AuthSession{}, nil, code, message, false
	}
	return h.contractInvitationSession(ctx, r, user, result.Organization.ID, audience)
}

func (h *APIHandler) contractInvitationSession(ctx context.Context, r *http.Request, user domain.User, organizationID contract.OrganizationID, audience token.Audience) (contract.AuthSession, http.Header, response.ErrorCode, string, bool) {
	now := time.Now().UTC()
	accessToken, _, err := h.Session.CreateSession(ctx, user.ID, audience, r.UserAgent(), now)
	if err != nil {
		code, message := invitationSessionError(err)
		return contract.AuthSession{}, nil, code, message, false
	}
	identity, err := h.Session.ValidateAccessToken(ctx, accessToken, audience, now)
	if err != nil {
		return contract.AuthSession{}, nil, response.CodeInternalError, "Session error.", false
	}
	accessToken, refreshToken, err := h.Session.SwitchSessionOrganization(ctx, user.ID, identity.SessionID, organizationID, audience, now)
	if err != nil {
		code, message := invitationSessionError(err)
		return contract.AuthSession{}, nil, code, message, false
	}
	return toContractAuthSession(user, accessToken, refreshToken), sessionHeader(h, accessToken, refreshToken), "", "", true
}

func sessionHeader(h *APIHandler, accessToken string, refreshToken string) http.Header {
	header := make(http.Header)
	session.SetAccessToken(contract.HeaderWriter(header), accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(contract.HeaderWriter(header), refreshToken, int(h.RefreshTTL.Seconds()))
	return header
}

func copyHeaders(dst http.Header, src http.Header) {
	for name, values := range src {
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

func toContractInvitationPreview(preview organization.InvitationPreview, now time.Time) contract.InvitationPreview {
	metadata := map[string]any{}
	if len(preview.Invitation.Metadata) > 0 {
		_ = json.Unmarshal(preview.Invitation.Metadata, &metadata)
	}
	return contract.InvitationPreview{
		Invitation: contract.OrganizationInvitation{
			Id:             preview.Invitation.ID,
			OrganizationId: preview.Invitation.OrganizationID,
			Email:          openapi_types.Email(preview.Invitation.Email),
			Role:           contract.OrganizationRole(preview.Invitation.Role),
			Metadata:       metadata,
			Status:         contract.OrganizationInvitationStatus(preview.Invitation.Status(now)),
			ExpiresAt:      preview.Invitation.ExpiresAt.UTC(),
		},
		Organization: contract.Organization{
			Id:              preview.Organization.ID,
			CreatedAt:       preview.Organization.CreatedAt,
			UpdatedAt:       preview.Organization.UpdatedAt,
			Name:            preview.Organization.Name,
			Kind:            contract.OrganizationKind(preview.Organization.Kind),
			CreatedByUserId: preview.Organization.CreatedByUserID,
		},
	}
}

func invitationError(err error) (response.ErrorCode, string) {
	switch {
	case errors.Is(err, organization.ErrInvalidOrganizationInvitationToken):
		return response.CodeInvalidRequest, "Invalid invitation token."
	case errors.Is(err, store.ErrOrganizationInvitationNotFound):
		return codeInvitationNotFound, "Invitation not found."
	case errors.Is(err, organization.ErrOrganizationInviteForbidden):
		return response.CodeForbidden, "Organization invitations are disabled."
	case errors.Is(err, organization.ErrOrganizationInviteEmailMismatch):
		return codeInvitationEmailMismatch, "This invitation is for a different account."
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted):
		return codeInvitationAlreadyAccepted, "Invitation already accepted."
	case errors.Is(err, organization.ErrOrganizationInvitationRevoked):
		return codeInvitationRevoked, "Invitation revoked."
	case errors.Is(err, organization.ErrOrganizationInvitationExpired):
		return codeInvitationExpired, "Invitation expired."
	case errors.Is(err, organization.ErrOrganizationSingleMembershipConflict):
		return codeOrganizationMembershipConflict, "Account already belongs to another organization."
	default:
		return response.CodeInternalError, "Invitation error."
	}
}

func invitationOrGoogleError(err error) (response.ErrorCode, string) {
	code, message := invitationError(err)
	if code != response.CodeInternalError {
		return code, message
	}
	code = googleLoginErrorCode(err)
	if code == response.CodeForbidden {
		return code, "Google login is not allowed for this account."
	}
	return code, "Google invitation login error."
}

func recoveryError(err error) (response.ErrorCode, string) {
	switch {
	case errors.Is(err, auth.ErrPendingProviderLinkInvalid):
		return response.CodeInvalidRequest, "Invalid provider link."
	case errors.Is(err, auth.ErrPendingProviderLinkExpired):
		return codeProviderLinkExpired, "Provider link expired."
	case errors.Is(err, auth.ErrInvalidCredentials):
		return response.CodeUnauthorized, "Invalid account proof."
	case errors.Is(err, auth.ErrPasswordProviderMissing):
		return codePasswordProviderNotFound, "Password authentication is not available for this account."
	case errors.Is(err, auth.ErrAuthProviderAlreadyLinked), errors.Is(err, auth.ErrAuthProviderAlreadyLinkedToUser):
		return codeAuthMethodAlreadyLinked, "Authentication method already linked."
	case errors.Is(err, auth.ErrProviderDisabled), errors.Is(err, auth.ErrProviderEmailNotVerified):
		return response.CodeForbidden, "Provider linking is not allowed."
	case errors.Is(err, store.ErrUserNotFound):
		return response.CodeNotFound, "Account not found."
	default:
		return response.CodeInternalError, "Account recovery error."
	}
}

func invitationSessionError(err error) (response.ErrorCode, string) {
	switch sessionErrorCode(err) {
	case response.CodeForbidden:
		return response.CodeForbidden, "Organization session forbidden."
	case response.CodeInternalError:
		return response.CodeInternalError, "Session error."
	default:
		return response.CodeUnauthorized, "Unauthorized."
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
