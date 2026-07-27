package openapi

import (
	"context"
	"encoding/json"
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

// ValidationMiddleware validates every public and internal API request and
// response against the document embedded in generated.go.
func ValidationMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	document, err := GetSwagger()
	if err != nil {
		panic("load embedded OpenAPI contract: " + err.Error())
	}
	document.Servers = nil

	router, err := legacyrouter.NewRouter(document)
	if err != nil {
		panic("build OpenAPI contract router: " + err.Error())
	}

	validator := openapi3filter.NewValidator(
		router,
		openapi3filter.Strict(true),
		openapi3filter.ValidationOptions(openapi3filter.Options{
			AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
			IncludeResponseStatus: true,
			SkipSettingDefaults:   true,
		}),
		openapi3filter.OnLog(func(ctx context.Context, message string, err error) {
			logger.ErrorContext(ctx, message, "error", err)
		}),
		openapi3filter.OnErr(func(_ context.Context, w http.ResponseWriter, status int, code openapi3filter.ErrCode, _ error) {
			errorCode := response.CodeInternalError
			message := "Response does not match the API contract."
			if code == openapi3filter.ErrCodeCannotFindRoute {
				errorCode = response.CodeNotFound
				message = "Route not found."
			} else if code == openapi3filter.ErrCodeRequestInvalid {
				errorCode = response.CodeInvalidRequest
				message = "Request does not match the API contract."
			}
			response.ErrorJSON(w, status, errorCode, message)
		}),
	)

	validate := validator.Middleware
	return func(next http.Handler) http.Handler {
		validated := validate(validateErrorCodes(router, next))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/auth/api/") || strings.HasPrefix(r.URL.Path, "/auth/internal/") {
				r.Body = http.MaxBytesReader(w, r.Body, maxAPIRequestBodyBytes)
				validated.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func validateErrorCodes(router routers.Router, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, r)

		if recorder.Code >= 400 {
			route, _, err := router.FindRoute(r)
			if err != nil || !responseCodeAllowed(route.Operation.Extensions, recorder.Code, recorder.Body.Bytes()) {
				response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Response does not match the API contract.")
				return
			}
		}

		for name, values := range recorder.Header() {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.WriteHeader(recorder.Code)
		_, _ = w.Write(recorder.Body.Bytes())
	})
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
