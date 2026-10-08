package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestSPACreateOrganizationUsesAuthenticatedUserAsInternalActor(t *testing.T) {
	internalCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/api/v1/user":
			assertSPASessionCookie(t, r)
			writeTestJSON(w, http.StatusOK, map[string]any{
				"id": "user-1", "email": "user@example.com", "username": "user",
			})
		case "/auth/api/v1/reauthenticate/check":
			assertSPACSRFRequest(t, r)
			w.WriteHeader(http.StatusNoContent)
		case "/auth/internal/v1/organizations":
			internalCalls++
			if got := r.Header.Get("Authorization"); got != "Bearer internal-secret" {
				t.Errorf("Authorization = %q", got)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["created_by_user_id"] != "user-1" || body["name"] != "Example" {
				t.Errorf("body = %#v", body)
			}
			writeTestJSON(w, http.StatusCreated, map[string]any{
				"organization": map[string]string{"id": "org-1", "name": "Example", "kind": "team"},
			})
		default:
			t.Fatalf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer upstream.Close()
	t.Setenv("AUTHARA_BASE_URL", upstream.URL)
	t.Setenv("AUTHARA_INTERNAL_API_TOKEN", "internal-secret")

	recorder := httptest.NewRecorder()
	request := newSPAMutationRequest(http.MethodPost, "/spa/api/v1/organizations", `{"name":"Example"}`)
	spaTestRouter().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if internalCalls != 1 {
		t.Fatalf("internal calls = %d, want 1", internalCalls)
	}
}

func TestSPAInternalMutationPassesThroughRecentAuthenticationChallenge(t *testing.T) {
	internalCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/api/v1/user":
			writeTestJSON(w, http.StatusOK, map[string]string{"id": "user-1"})
		case "/auth/api/v1/reauthenticate/check":
			writeTestJSON(w, http.StatusPreconditionRequired, map[string]any{
				"error": map[string]string{
					"code": "recent_authentication_required", "message": "Authenticate again.",
				},
				"authentication_challenge": map[string]string{
					"id": "challenge-1", "expires_at": "2026-09-24T12:00:00Z",
				},
			})
		default:
			internalCalled = true
		}
	}))
	defer upstream.Close()
	t.Setenv("AUTHARA_BASE_URL", upstream.URL)
	t.Setenv("AUTHARA_INTERNAL_API_TOKEN", "internal-secret")

	recorder := httptest.NewRecorder()
	request := newSPAMutationRequest(http.MethodDelete, "/spa/api/v1/organizations/org-1", "")
	spaTestRouter().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if internalCalled {
		t.Fatal("internal API was called before recent authentication")
	}
	if !strings.Contains(recorder.Body.String(), `"id":"challenge-1"`) {
		t.Fatalf("challenge response was not preserved: %s", recorder.Body.String())
	}
}

func TestSPAInternalMutationRoutesUseServerSideActor(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		path             string
		body             string
		wantInternalPath string
		wantMethod       string
		wantStatus       int
		managerReadPath  string
		assertBody       func(*testing.T, map[string]any)
	}{
		{
			name: "delete organization", method: http.MethodDelete, path: "/spa/api/v1/organizations/org-1",
			wantInternalPath: "/auth/internal/v1/organizations/org-1", wantMethod: http.MethodDelete, wantStatus: http.StatusNoContent,
			assertBody: assertActorBody,
		},
		{
			name: "remove member", method: http.MethodDelete, path: "/spa/api/v1/organizations/org-1/members/user-2",
			wantInternalPath: "/auth/internal/v1/organizations/org-1/members/user-2", wantMethod: http.MethodDelete, wantStatus: http.StatusNoContent,
			assertBody: assertActorBody,
		},
		{
			name: "transfer ownership", method: http.MethodPost, path: "/spa/api/v1/organizations/org-1/ownership-transfer", body: `{"new_owner_user_id":"user-2"}`,
			wantInternalPath: "/auth/internal/v1/organizations/org-1/ownership-transfer", wantMethod: http.MethodPost, wantStatus: http.StatusNoContent,
			assertBody: func(t *testing.T, body map[string]any) {
				assertActorBody(t, body)
				if body["new_owner_user_id"] != "user-2" {
					t.Errorf("new_owner_user_id = %#v", body["new_owner_user_id"])
				}
			},
		},
		{
			name: "create invitation", method: http.MethodPost, path: "/spa/api/v1/organizations/org-1/invitations", body: `{"email":"invitee@example.com","role":"admin"}`,
			wantInternalPath: "/auth/internal/v1/organizations/org-1/invitations", wantMethod: http.MethodPost, wantStatus: http.StatusCreated,
			assertBody: func(t *testing.T, body map[string]any) {
				assertActorBody(t, body)
				if body["email"] != "invitee@example.com" || body["role"] != "admin" {
					t.Errorf("invitation body = %#v", body)
				}
			},
		},
		{
			name: "resend invitation", method: http.MethodPost, path: "/spa/api/v1/organizations/org-1/invitations/inv-1/resend",
			wantInternalPath: "/auth/internal/v1/organizations/org-1/invitations/inv-1/resend", wantMethod: http.MethodPost, wantStatus: http.StatusCreated,
			managerReadPath: "/auth/api/v1/organizations/org-1/invitations/inv-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/auth/api/v1/user":
					writeTestJSON(w, http.StatusOK, map[string]string{"id": "user-1"})
				case "/auth/api/v1/reauthenticate/check":
					w.WriteHeader(http.StatusNoContent)
				case tt.managerReadPath:
					if tt.managerReadPath == "" {
						t.Fatalf("unexpected public authorization read")
					}
					writeTestJSON(w, http.StatusOK, map[string]any{"invitation": map[string]string{"id": "inv-1"}})
				case tt.wantInternalPath:
					if r.Method != tt.wantMethod {
						t.Errorf("method = %s, want %s", r.Method, tt.wantMethod)
					}
					if r.Header.Get("Authorization") != "Bearer internal-secret" {
						t.Errorf("missing internal authorization")
					}
					if tt.assertBody != nil {
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Fatal(err)
						}
						tt.assertBody(t, body)
					}
					if tt.wantStatus == http.StatusCreated {
						writeTestJSON(w, tt.wantStatus, map[string]any{"invitation": map[string]string{"id": "inv-2"}})
					} else {
						w.WriteHeader(tt.wantStatus)
					}
				default:
					t.Fatalf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
				}
			}))
			defer upstream.Close()
			t.Setenv("AUTHARA_BASE_URL", upstream.URL)
			t.Setenv("AUTHARA_INTERNAL_API_TOKEN", "internal-secret")

			recorder := httptest.NewRecorder()
			request := newSPAMutationRequest(tt.method, tt.path, tt.body)
			spaTestRouter().ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestSPADeleteCurrentUserCanOnlyTargetAuthenticatedUser(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/api/v1/user":
			writeTestJSON(w, http.StatusOK, map[string]string{"id": "session-user"})
		case "/auth/api/v1/reauthenticate/check":
			w.WriteHeader(http.StatusNoContent)
		case "/auth/internal/v1/users/session-user":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer upstream.Close()
	t.Setenv("AUTHARA_BASE_URL", upstream.URL)
	t.Setenv("AUTHARA_INTERNAL_API_TOKEN", "internal-secret")

	recorder := httptest.NewRecorder()
	request := newSPAMutationRequest(http.MethodDelete, "/spa/api/v1/account", "")
	spaTestRouter().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 || cookies[0].MaxAge != -1 || cookies[1].MaxAge != -1 {
		t.Fatalf("cleared cookies = %#v", cookies)
	}
}

func TestSPAInternalMutationRejectsMissingCSRFBeforeCallingAuthara(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/spa/api/v1/account", nil)
	spaTestRouter().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func spaTestRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(RequireSPACSRF)
	r.Post("/spa/api/v1/organizations", SPACreateOrganization)
	r.Delete("/spa/api/v1/organizations/{organizationID}", SPADeleteOrganization)
	r.Delete("/spa/api/v1/organizations/{organizationID}/members/{userID}", SPARemoveOrganizationMember)
	r.Post("/spa/api/v1/organizations/{organizationID}/ownership-transfer", SPATransferOrganizationOwnership)
	r.Post("/spa/api/v1/organizations/{organizationID}/invitations", SPACreateOrganizationInvitation)
	r.Post("/spa/api/v1/organizations/{organizationID}/invitations/{invitationID}/resend", SPAResendOrganizationInvitation)
	r.Delete("/spa/api/v1/account", SPADeleteCurrentUser)
	return r
}

func newSPAMutationRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf-token")
	request.AddCookie(&http.Cookie{Name: "authara_access", Value: "session"})
	request.AddCookie(&http.Cookie{Name: "authara_csrf", Value: "csrf-token"})
	return request
}

func assertSPASessionCookie(t *testing.T, r *http.Request) {
	t.Helper()
	cookie, err := r.Cookie("authara_access")
	if err != nil || cookie.Value != "session" {
		t.Errorf("access cookie = %#v, %v", cookie, err)
	}
}

func assertSPACSRFRequest(t *testing.T, r *http.Request) {
	t.Helper()
	assertSPASessionCookie(t, r)
	if r.Header.Get("X-CSRF-Token") != "csrf-token" {
		t.Errorf("X-CSRF-Token = %q", r.Header.Get("X-CSRF-Token"))
	}
}

func assertActorBody(t *testing.T, body map[string]any) {
	t.Helper()
	if body["actor_user_id"] != "user-1" {
		t.Errorf("actor_user_id = %#v, want user-1", body["actor_user_id"])
	}
}

func writeTestJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
