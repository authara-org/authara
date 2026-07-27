package http

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/handlers/api"
	"github.com/authara-org/authara/internal/http/handlers/internalapi"
	"github.com/authara-org/authara/internal/http/kit/response"
	openapicontract "github.com/authara-org/authara/internal/http/openapi"
)

type openAPIServer struct {
	*api.APIHandler
	*internalapi.Handler
}

var _ openapicontract.StrictServerInterface = (*openAPIServer)(nil)

func newOpenAPIServer(handlers Handlers) *openapicontract.ServerInterfaceWrapper {
	server := &openAPIServer{
		APIHandler: handlers.API,
		Handler:    handlers.InternalAPI,
	}
	strict := openapicontract.NewStrictHandlerWithOptions(
		server,
		[]openapicontract.StrictMiddlewareFunc{openapicontract.CaptureRequest},
		openapicontract.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
				response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Request does not match the API contract.")
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
				response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Response does not match the API contract.")
			},
		},
	)
	return &openapicontract.ServerInterfaceWrapper{
		Handler: strict,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
			response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Request does not match the API contract.")
		},
	}
}
