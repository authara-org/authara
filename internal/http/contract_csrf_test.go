package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/http/middleware"
	openapicontract "github.com/authara-org/authara/internal/http/openapi"
)

const markerAPICSRF = 432

func TestOpenAPICSRFWiring(t *testing.T) {
	document, err := openapicontract.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI contract: %v", err)
	}
	router := newCSRFMarkerRouter()

	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			required := operation.Extensions["x-authara-csrf"] == "required"
			t.Run(operationKey(method, path), func(t *testing.T) {
				req := httptest.NewRequest(method, materializeRoutePath(path), nil)
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)

				if required && rr.Code != markerAPICSRF {
					t.Fatalf("OpenAPI requires CSRF but middleware marker was not reached; got %d", rr.Code)
				}
				if !required && rr.Code == markerAPICSRF {
					t.Fatal("route has CSRF middleware but OpenAPI does not declare it")
				}
			})
		}
	}
}

func TestOpenAPICSRFErrorShape(t *testing.T) {
	router := newRealCSRFRouter()
	req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/passkeys/authenticate/options", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode API CSRF response: %v", err)
	}
	if body.Error.Code != "forbidden" {
		t.Fatalf("expected forbidden, got %q", body.Error.Code)
	}
}

func newCSRFMarkerRouter() http.Handler {
	marker := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(markerAPICSRF)
		})
	}
	return newCSRFContractRouter(marker, false)
}

func newRealCSRFRouter() http.Handler {
	return newCSRFContractRouter(middleware.RequireAPICSRF, true)
}

func newCSRFContractRouter(apiCSRF func(http.Handler) http.Handler, validateOpenAPI bool) http.Handler {
	pass := func(next http.Handler) http.Handler { return next }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := ServerConfig{
		Version:                  "test",
		Logger:                   logger,
		Handlers:                 newTestHandlers(logger, render.New(render.Assets{}, false)),
		disableOpenAPIValidation: !validateOpenAPI,
	}
	mw := Middlewares{
		RedirectIfAuthenticated:             pass,
		RequireAppAccessAuthWithRefresh:     pass,
		RequireAppAccessAuthAPI:             pass,
		RequireAdminAccessAuthWithRefresh:   pass,
		RequireAdminAccessAuthAPI:           pass,
		RequireInternalAPIAuth:              pass,
		RequirePublicOrganizationManagement: pass,
		RequireAdminRole:                    pass,
		RequireCSRF:                         pass,
		RequireAPICSRF:                      apiCSRF,
		ReturnTo:                            pass,
		HTMX:                                pass,
		RequireChallengeEnabled:             pass,
		RequireAllowlistEnabled:             pass,
		OptionalAppAccessIdentity:           pass,
	}
	return NewRouter(cfg, mw)
}
