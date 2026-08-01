package api

import (
	"context"
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/csrf"
	contract "github.com/authara-org/authara/internal/http/openapi"
)

func (h *APIHandler) GetCsrfToken(ctx context.Context, _ contract.GetCsrfTokenRequestObject) (contract.GetCsrfTokenResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return getCsrfTokenError(responseCodeInternalError(), "API contract error."), nil
	}
	header := make(http.Header)
	token, err := csrf.EnsureCookie(contract.HeaderWriter(header), r)
	if err != nil {
		return getCsrfTokenError(responseCodeInternalError(), "CSRF token error."), nil
	}
	return contract.GetCsrfToken200HeadersResponse{Header: header, Body: contract.CSRFToken{CsrfToken: token}}, nil
}
