package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/handlers/internalapi"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/organization"
)

func TestOpenAPIRouterContractFailures(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		path             string
		body             string
		header           http.Header
		organizationMode organization.OrgMode
		want             int
		code             string
	}{
		{
			name:   "rejects unsupported api method",
			method: http.MethodGet,
			path:   "/auth/api/v1/login",
			want:   http.StatusNotFound,
			code:   "not_found",
		},
		{
			name:   "rejects malformed json before handler",
			method: http.MethodPost,
			path:   "/auth/api/v1/login",
			body:   `{`,
			header: http.Header{"Content-Type": {"application/json"}},
			want:   http.StatusBadRequest,
			code:   "invalid_request",
		},
		{
			name:   "rejects legacy password login email field",
			method: http.MethodPost,
			path:   "/auth/api/v1/login",
			body:   `{"email":"user@example.com","password":"password123"}`,
			header: http.Header{"Content-Type": {"application/json"}},
			want:   http.StatusBadRequest,
			code:   "invalid_request",
		},
		{
			name:   "rejects invalid path uuid",
			method: http.MethodGet,
			path:   "/auth/api/v1/organizations/not-a-uuid",
			want:   http.StatusBadRequest,
			code:   "invalid_request",
		},
		{
			name:   "rejects wrong content type",
			method: http.MethodPost,
			path:   "/auth/api/v1/tokens/refresh",
			body:   `{"refresh_token":"x"}`,
			header: http.Header{"Content-Type": {"text/plain"}},
			want:   http.StatusBadRequest,
			code:   "invalid_request",
		},
		{
			name:             "rejects invalid response",
			method:           http.MethodGet,
			path:             "/auth/api/v1/capabilities",
			organizationMode: "invalid",
			want:             http.StatusInternalServerError,
			code:             "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newOpenAPIContractTestRouter(tt.organizationMode)
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			for name, values := range tt.header {
				for _, value := range values {
					req.Header.Add(name, value)
				}
			}
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assertRouterContractError(t, rr, tt.want, tt.code)
		})
	}
}

func newOpenAPIContractTestRouter(mode organization.OrgMode) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pass := func(next http.Handler) http.Handler { return next }
	handlers := newTestHandlers(logger, render.New(render.Assets{}, false))
	if mode != "" {
		handlers.InternalAPI = internalapi.New(nil, organization.New(organization.Config{Mode: mode}), false)
	}

	return NewRouter(ServerConfig{
		Version:  "test",
		Logger:   logger,
		Handlers: handlers,
	}, Middlewares{
		RedirectIfAuthenticated:             pass,
		RequireAppAccessAuthWithRefresh:     pass,
		RequireAppAccessAuthAPI:             pass,
		RequireAdminAccessAuthWithRefresh:   pass,
		RequireAdminAccessAuthAPI:           pass,
		RequireInternalAPIAuth:              pass,
		RequirePublicOrganizationManagement: pass,
		RequireAdminRole:                    pass,
		RequireCSRF:                         pass,
		RequireAPICSRF:                      pass,
		ReturnTo:                            pass,
		HTMX:                                pass,
		RequireChallengeEnabled:             pass,
		RequireAllowlistEnabled:             pass,
		OptionalAppAccessIdentity:           pass,
	})
}

func assertRouterContractError(t *testing.T, rr *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rr.Code != status {
		t.Fatalf("expected %d, got %d: %s", status, rr.Code, rr.Body.String())
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("expected error code %q, got %q", code, body.Error.Code)
	}
}
