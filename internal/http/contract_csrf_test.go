package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/http/middleware"
	"gopkg.in/yaml.v3"
)

type httpContract struct {
	Routes       []contractRoute `yaml:"routes"`
	CSRFContract csrfContract    `yaml:"csrf_contract"`
}

type csrfContract struct {
	Stability  string            `yaml:"stability"`
	AppliesTo  csrfAppliesTo     `yaml:"applies_to"`
	BrowserErr csrfErrorContract `yaml:"browser_error"`
	APIErr     csrfAPIError      `yaml:"api_error"`
}

type csrfAppliesTo struct {
	Rule       string   `yaml:"rule"`
	Exceptions []string `yaml:"exceptions"`
}

type csrfErrorContract struct {
	Status       int    `yaml:"status"`
	ResponseKind string `yaml:"response_kind"`
}

type csrfAPIError struct {
	Status       int              `yaml:"status"`
	ResponseKind string           `yaml:"response_kind"`
	Error        csrfAPIErrorBody `yaml:"error"`
}

type csrfAPIErrorBody struct {
	Code string `yaml:"code"`
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

const (
	markerBrowserCSRF = 431
	markerAPICSRF     = 432
)

func loadHTTPContract(t *testing.T) httpContract {
	t.Helper()

	data, err := os.ReadFile("../../contract/http.yaml")
	if err != nil {
		t.Fatalf("read contract/http.yaml: %v", err)
	}

	var contract httpContract
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatalf("unmarshal contract/http.yaml: %v", err)
	}

	return contract
}

func newCSRFFocusedContractTestRouter() http.Handler {
	pass := func(next http.Handler) http.Handler { return next }

	cfg := ServerConfig{
		Version: "test",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Render:  render.New(render.Assets{}),
	}

	mw := Middlewares{
		RedirectIfAuthenticated:           pass,
		RequireAppAccessAuthWithRefresh:   pass,
		RequireAppAccessAuthAPI:           pass,
		RequireAdminAccessAuthWithRefresh: pass,
		RequireAdminRole:                  pass,
		ReturnTo:                          pass,

		// real CSRF middlewares
		RequireCSRF:    middleware.RequireCSRF,
		RequireAPICSRF: middleware.RequireAPICSRF,
	}

	return NewRouter(cfg, mw)
}

func newCSRFWiringContractTestRouter() http.Handler {
	pass := func(next http.Handler) http.Handler { return next }

	marker := func(status int, body string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, body, status)
			})
		}
	}

	cfg := ServerConfig{
		Version: "test",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Render:  render.New(render.Assets{}),
	}

	mw := Middlewares{
		RedirectIfAuthenticated:           pass,
		RequireAppAccessAuthWithRefresh:   pass,
		RequireAppAccessAuthAPI:           pass,
		RequireAdminAccessAuthWithRefresh: pass,
		RequireAdminRole:                  pass,
		ReturnTo:                          pass,

		RequireCSRF:    marker(markerBrowserCSRF, "browser-csrf"),
		RequireAPICSRF: marker(markerAPICSRF, "api-csrf"),
	}

	return NewRouter(cfg, mw)
}

func routeShouldHaveCSRF(route contractRoute, exceptions []string) bool {
	if route.Stability != "stable" {
		return false
	}
	if route.Method != http.MethodPost {
		return false
	}
	if !strings.HasPrefix(route.Path, "/auth") {
		return false
	}
	if slices.Contains(exceptions, route.Path) {
		return false
	}
	return true
}

func TestCSRFContract_StableAuthPostRoutesRequireCSRF(t *testing.T) {
	contract := loadHTTPContract(t)
	r := newCSRFFocusedContractTestRouter()

	exceptions := contract.CSRFContract.AppliesTo.Exceptions

	for _, route := range contract.Routes {
		if !routeShouldHaveCSRF(route, exceptions) {
			continue
		}

		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			req := httptest.NewRequest(route.Method, materializeRoutePath(route.Path), nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if strings.HasPrefix(route.Path, "/auth/api/v1") {
				if rr.Code != contract.CSRFContract.APIErr.Status {
					t.Fatalf("expected status %d, got %d", contract.CSRFContract.APIErr.Status, rr.Code)
				}

				var env errorEnvelope
				if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
					t.Fatalf("expected valid JSON error envelope: %v", err)
				}
				if env.Error.Code != contract.CSRFContract.APIErr.Error.Code {
					t.Fatalf("expected error code %q, got %q", contract.CSRFContract.APIErr.Error.Code, env.Error.Code)
				}
				if env.Error.Message == "" {
					t.Fatal("expected non-empty error message")
				}
			} else {
				if rr.Code != contract.CSRFContract.BrowserErr.Status {
					t.Fatalf("expected status %d, got %d", contract.CSRFContract.BrowserErr.Status, rr.Code)
				}
			}
		})
	}
}

func TestCSRFContract_OnlyExpectedRoutesHaveCSRFMiddleware(t *testing.T) {
	contract := loadHTTPContract(t)
	r := newCSRFWiringContractTestRouter()

	exceptions := contract.CSRFContract.AppliesTo.Exceptions

	for _, route := range contract.Routes {
		if route.Stability != "stable" {
			continue
		}
		if !strings.HasPrefix(route.Path, "/auth") {
			continue
		}

		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			req := httptest.NewRequest(route.Method, materializeRoutePath(route.Path), nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			gotCSRF := rr.Code == markerBrowserCSRF || rr.Code == markerAPICSRF
			wantCSRF := routeShouldHaveCSRF(route, exceptions)

			if wantCSRF && !gotCSRF {
				t.Fatalf("expected CSRF middleware for %s %s, got status %d", route.Method, route.Path, rr.Code)
			}

			if !wantCSRF && gotCSRF {
				t.Fatalf("route %s %s unexpectedly has CSRF middleware", route.Method, route.Path)
			}

			if wantCSRF {
				if strings.HasPrefix(route.Path, "/auth/api/v1") && rr.Code != markerAPICSRF {
					t.Fatalf("expected API CSRF middleware for %s %s, got status %d", route.Method, route.Path, rr.Code)
				}

				if !strings.HasPrefix(route.Path, "/auth/api/v1") && rr.Code != markerBrowserCSRF {
					t.Fatalf("expected browser CSRF middleware for %s %s, got status %d", route.Method, route.Path, rr.Code)
				}
			}
		})
	}
}

func TestCSRFContract_BrowserMiddlewareReturnsForbiddenText(t *testing.T) {
	contract := loadHTTPContract(t)

	handler := middleware.RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected middleware to block request")
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/sessions/logout", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != contract.CSRFContract.BrowserErr.Status {
		t.Fatalf("expected status %d, got %d", contract.CSRFContract.BrowserErr.Status, rr.Code)
	}

	if contract.CSRFContract.BrowserErr.ResponseKind != "text" {
		t.Fatalf("expected browser response_kind text, got %q", contract.CSRFContract.BrowserErr.ResponseKind)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parse Content-Type: %v", err)
		}
		if mediaType == "application/json" {
			t.Fatalf("expected non-JSON browser CSRF error, got %q", mediaType)
		}
	}
}

func TestCSRFContract_APIMiddlewareReturnsForbiddenJSON(t *testing.T) {
	contract := loadHTTPContract(t)

	handler := middleware.RequireAPICSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected middleware to block request")
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/sessions/refresh", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != contract.CSRFContract.APIErr.Status {
		t.Fatalf("expected status %d, got %d", contract.CSRFContract.APIErr.Status, rr.Code)
	}

	if contract.CSRFContract.APIErr.ResponseKind != "json" {
		t.Fatalf("expected api response_kind json, got %q", contract.CSRFContract.APIErr.ResponseKind)
	}

	ct := rr.Header().Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("parse Content-Type: %v", err)
	}
	if mediaType != "application/json" {
		t.Fatalf("expected application/json, got %q", mediaType)
	}

	var env errorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("expected valid JSON error envelope: %v", err)
	}

	if env.Error.Code != contract.CSRFContract.APIErr.Error.Code {
		t.Fatalf("expected error code %q, got %q", contract.CSRFContract.APIErr.Error.Code, env.Error.Code)
	}
	if env.Error.Message == "" {
		t.Fatal("expected non-empty error message")
	}
}
