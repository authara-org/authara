package api

import (
	"context"
	"net/http"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func contractRequest(ctx context.Context) (*http.Request, bool) {
	r, ok := contract.Request(ctx)
	if !ok {
		return nil, false
	}
	return r, true
}

func appAudience[T ~string](audience *T) (token.Audience, response.ErrorCode, string, bool) {
	if audience != nil && string(*audience) != string(token.AudienceApp) {
		return "", responseCodeForbidden(), "Signup only supports app audience.", false
	}
	return token.AudienceApp, "", "", true
}

func currentOrganization(ctx context.Context) (openapi_types.UUID, domain.OrganizationRole, response.ErrorCode, string, bool) {
	organizationID, ok := httpctx.OrganizationID(ctx)
	if !ok {
		return openapi_types.UUID{}, "", response.CodeUnauthorized, "Unauthorized", false
	}
	role, ok := httpctx.OrganizationRole(ctx)
	if !ok {
		return openapi_types.UUID{}, "", response.CodeUnauthorized, "Unauthorized", false
	}
	return organizationID, role, "", "", true
}

func toContractAuthSession(user domain.User, accessToken, refreshToken string, states ...contract.OrganizationSessionState) contract.AuthSession {
	state := contract.OrganizationSessionState{Status: contract.Active}
	if len(states) != 0 {
		state = states[0]
	}
	return contract.AuthSession{
		User: contract.AuthUser{
			Id:              user.ID,
			Email:           openapi_types.Email(user.Email),
			EmailVerified:   user.EmailVerifiedAt != nil,
			EmailVerifiedAt: user.EmailVerifiedAt,
			Username:        user.Username,
			Disabled:        user.DisabledAt != nil,
			CreatedAt:       user.CreatedAt,
		},
		AccessToken:       accessToken,
		RefreshToken:      refreshToken,
		OrganizationState: state,
	}
}

func (h *APIHandler) organizationSessionState(activeOrganizationID uuid.UUID) contract.OrganizationSessionState {
	mode := organization.OrgMode("")
	if h.Organizations != nil {
		mode = h.Organizations.Mode()
	}
	if activeOrganizationID == uuid.Nil {
		return contract.OrganizationSessionState{
			Status:                contract.OrganizationRequired,
			CanAcceptInvitation:   true,
			CanCreateOrganization: mode == organization.OrgModeSingle,
		}
	}
	return contract.OrganizationSessionState{
		Status:                contract.Active,
		CanAcceptInvitation:   mode == organization.OrgModeMulti,
		CanCreateOrganization: mode == organization.OrgModeMulti,
	}
}

func toContractCookieAuthSession(user domain.User, states ...contract.OrganizationSessionState) contract.AuthSession {
	return toContractAuthSession(user, "", "", states...)
}

func toContractOrganizationSummary(org domain.Organization, role domain.OrganizationRole) contract.OrganizationSummary {
	return contract.OrganizationSummary{
		Id:   org.ID,
		Name: org.Name,
		Role: contract.OrganizationRole(role),
	}
}

func responseCodeInvalidRequest() response.ErrorCode { return response.CodeInvalidRequest }
func responseCodeUnauthorized() response.ErrorCode   { return response.CodeUnauthorized }
func responseCodeForbidden() response.ErrorCode      { return response.CodeForbidden }
func responseCodeNotFound() response.ErrorCode       { return response.CodeNotFound }
func responseCodeRateLimited() response.ErrorCode    { return response.CodeRateLimited }
func responseCodeInternalError() response.ErrorCode  { return response.CodeInternalError }
