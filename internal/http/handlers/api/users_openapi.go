package api

import (
	"context"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *APIHandler) GetCurrentUser(ctx context.Context, _ contract.GetCurrentUserRequestObject) (contract.GetCurrentUserResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return getCurrentUserError(responseCodeUnauthorized(), "Unauthorized"), nil
	}
	userRoles, ok := httpctx.Roles(ctx)
	if !ok {
		return getCurrentUserError(responseCodeUnauthorized(), "Unauthorized"), nil
	}
	organizationID, organizationRole, code, message, ok := currentOrganization(ctx)
	if !ok {
		return getCurrentUserError(code, message), nil
	}
	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		return getCurrentUserError(responseCodeUnauthorized(), "Unauthorized"), nil
	}
	org, err := h.Organizations.GetOrganization(ctx, organizationID)
	if err != nil {
		return getCurrentUserError(responseCodeUnauthorized(), "Unauthorized"), nil
	}
	roles := make([]contract.CurrentUserRoles, 0, len(userRoles.List()))
	for _, role := range userRoles.List() {
		roles = append(roles, contract.CurrentUserRoles(role))
	}
	return contract.GetCurrentUser200JSONResponse(contract.CurrentUser{
		Id:           user.ID,
		Email:        openapi_types.Email(user.Email),
		Username:     user.Username,
		Disabled:     user.DisabledAt != nil,
		CreatedAt:    user.CreatedAt,
		Roles:        roles,
		Organization: toContractOrganizationSummary(org, organizationRole),
	}), nil
}
