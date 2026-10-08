package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
)

const maxAPIRequestBodyBytes = 1 << 20

// ValidationMiddleware validates every public and internal API request against
// the document embedded in generated.go. Responses are validated only when
// validateResponses is true so production responses can be written directly.
func ValidationMiddleware(logger *slog.Logger, validateResponses bool) func(http.Handler) http.Handler {
	document, err := GetSwagger()
	if err != nil {
		panic("load embedded OpenAPI contract: " + err.Error())
	}
	document.Servers = nil

	router, err := legacyrouter.NewRouter(document)
	if err != nil {
		panic("build OpenAPI contract router: " + err.Error())
	}

	options := &openapi3filter.Options{
		AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
		IncludeResponseStatus: true,
		SkipSettingDefaults:   true,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/auth/api/") || strings.HasPrefix(r.URL.Path, "/auth/internal/") {
				r.Body = http.MaxBytesReader(w, r.Body, maxAPIRequestBodyBytes)
				validateAPIRequest(logger, router, options, next, validateResponses, w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func validateAPIRequest(
	logger *slog.Logger,
	router routers.Router,
	options *openapi3filter.Options,
	next http.Handler,
	validateResponse bool,
	w http.ResponseWriter,
	r *http.Request,
) {
	route, pathParams, err := router.FindRoute(r)
	if err != nil {
		logger.ErrorContext(r.Context(), "validation error: failed to find route for "+r.URL.String(), "error", err)
		response.ErrorJSON(w, http.StatusNotFound, response.CodeNotFound, "Route not found.")
		return
	}

	requestInput := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
		Options:    options,
	}
	if err := openapi3filter.ValidateRequest(r.Context(), requestInput); err != nil {
		logger.ErrorContext(r.Context(), "invalid request", "error", err)
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Request does not match the API contract.")
		return
	}

	if !validateResponse {
		next.ServeHTTP(w, r)
		return
	}

	validateAPIResponse(logger, requestInput, options, next, w, r)
}

func validateAPIResponse(
	logger *slog.Logger,
	requestInput *openapi3filter.RequestValidationInput,
	options *openapi3filter.Options,
	next http.Handler,
	w http.ResponseWriter,
	r *http.Request,
) {
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, r)

	if recorder.Code >= 400 && !responseCodeAllowed(requestInput.Route.Operation.Extensions, recorder.Code, recorder.Body.Bytes()) {
		err := fmt.Errorf("status %d contains an undeclared error code", recorder.Code)
		logger.ErrorContext(r.Context(), "invalid response", "error", err)
		response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Response does not match the API contract.")
		return
	}

	err := openapi3filter.ValidateResponse(r.Context(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestInput,
		Status:                 recorder.Code,
		Header:                 recorder.Header(),
		Body:                   io.NopCloser(bytes.NewReader(recorder.Body.Bytes())),
		Options:                options,
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "invalid response", "error", err)
		response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Response does not match the API contract.")
		return
	}

	for name, values := range recorder.Header() {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(recorder.Code)
	_, _ = w.Write(recorder.Body.Bytes())
}

func responseCodeAllowed(extensions map[string]any, status int, body []byte) bool {
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	return ErrorCodeAllowed(extensions, status, response.ErrorCode(envelope.Error.Code))
}
