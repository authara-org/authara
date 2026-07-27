package api

import (
	"context"
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/csrf"
	contract "github.com/authara-org/authara/internal/http/openapi"
)

func (h *APIHandler) GetCsrfToken(ctx context.Context, _ contract.GetCsrfTokenRequestObject) (contract.GetCsrfTokenResponseObject, error) {
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	header := make(http.Header)
	token, err := csrf.EnsureCookie(contract.HeaderWriter(header), r)
	if err != nil {
		return routeError(GetCsrfTokenErrors, responseCodeInternalError(), "CSRF token error."), nil
	}
	return contract.JSON(http.StatusOK, contract.CSRFToken{CsrfToken: token}, header), nil
}
