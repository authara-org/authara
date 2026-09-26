package api

import (
	"context"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/store"
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

func (h *APIHandler) SetCurrentUserPassword(ctx context.Context, request contract.SetCurrentUserPasswordRequestObject) (contract.SetCurrentUserPasswordResponseObject, error) {
	if request.Body == nil {
		return setCurrentUserPasswordError(responseCodeInvalidRequest(), "Invalid JSON body."), nil
	}
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return setCurrentUserPasswordError(responseCodeUnauthorized(), "Unauthorized"), nil
	}

	passwordHash, err := h.Auth.HashPassword(ctx, request.Body.Password)
	if err != nil {
		code, message := h.passwordPolicyError(err)
		return setCurrentUserPasswordError(code, message), nil
	}
	if err := h.Auth.SetPassword(ctx, userID, passwordHash, time.Now().UTC()); err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return setCurrentUserPasswordError(responseCodeUnauthorized(), "Unauthorized"), nil
		}
		return setCurrentUserPasswordError(responseCodeInternalError(), "Password error"), nil
	}

	return contract.SetCurrentUserPassword204Response{}, nil
}
