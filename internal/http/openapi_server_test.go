package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/render"
)

func TestOpenAPIServerBridgeValidatesSignupResponse(t *testing.T) {
	pass := func(next http.Handler) http.Handler { return next }
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handlers := newTestHandlers(logger, render.New(render.Assets{}, false))
	handlers.API.ChallengeEnabled = true

	router := NewRouter(ServerConfig{
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

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/api/v1/signup/direct",
		strings.NewReader(`{"email":"person@example.com","password":"password123"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s\n%s", rr.Code, rr.Body.String(), logs.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Fatalf("expected not_found, got %q", body.Error.Code)
	}
}
