package api

import (
	"context"
	"net/http"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/session/token"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func contractRequest(ctx context.Context) (*http.Request, contract.Response, bool) {
	r, ok := contract.Request(ctx)
	if !ok {
		return nil, contract.InternalError(), false
	}
	return r, contract.Response{}, true
}

func appAudience[T ~string](audience *T, routeErrors map[response.ErrorCode]response.ErrorSpec) (token.Audience, contract.Response, bool) {
	if audience != nil && string(*audience) != string(token.AudienceApp) {
		return "", routeError(routeErrors, responseCodeForbidden(), "Signup only supports app audience."), false
	}
	return token.AudienceApp, contract.Response{}, true
}

func currentOrganization(ctx context.Context, routeErrors map[response.ErrorCode]response.ErrorSpec) (openapi_types.UUID, domain.OrganizationRole, contract.Response, bool) {
	organizationID, ok := httpctx.OrganizationID(ctx)
	if !ok {
		return openapi_types.UUID{}, "", routeError(routeErrors, response.CodeUnauthorized, "Unauthorized"), false
	}
	role, ok := httpctx.OrganizationRole(ctx)
	if !ok {
		return openapi_types.UUID{}, "", routeError(routeErrors, response.CodeUnauthorized, "Unauthorized"), false
	}
	return organizationID, role, contract.Response{}, true
}

func toContractAuthSession(user domain.User, accessToken, refreshToken string) contract.AuthSession {
	return contract.AuthSession{
		User: contract.AuthUser{
			Id:        user.ID,
			Email:     openapi_types.Email(user.Email),
			Username:  user.Username,
			Disabled:  user.DisabledAt != nil,
			CreatedAt: user.CreatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
}

func toContractOrganizationSummary(org domain.Organization, role domain.OrganizationRole) contract.OrganizationSummary {
	return contract.OrganizationSummary{
		Id:   org.ID,
		Name: org.Name,
		Role: contract.OrganizationRole(role),
	}
}

func routeError(routeErrors map[response.ErrorCode]response.ErrorSpec, code response.ErrorCode, message string) contract.Response {
	spec := mustRouteError(routeErrors, code)
	return contract.ErrorJSON(spec.Status, spec.Code, message)
}

func responseCodeInvalidRequest() response.ErrorCode { return response.CodeInvalidRequest }
func responseCodeUnauthorized() response.ErrorCode   { return response.CodeUnauthorized }
func responseCodeForbidden() response.ErrorCode      { return response.CodeForbidden }
func responseCodeNotFound() response.ErrorCode       { return response.CodeNotFound }
func responseCodeRateLimited() response.ErrorCode    { return response.CodeRateLimited }
func responseCodeInternalError() response.ErrorCode  { return response.CodeInternalError }
