package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOrganizationReadsUseAuthenticatedPublicAPI(t *testing.T) {
	const (
		organizationID = "organization-1"
		userID         = "user-1"
	)

	wantedPaths := map[string]int{
		"/auth/api/v1/capabilities":                                           1,
		"/auth/api/v1/users/" + userID + "/memberships":                       1,
		"/auth/api/v1/organizations/" + organizationID:                        1,
		"/auth/api/v1/organizations/" + organizationID + "/members":           1,
		"/auth/api/v1/organizations/" + organizationID + "/members/" + userID: 1,
		"/auth/api/v1/organizations/" + organizationID + "/invitations":       1,
	}
	seenPaths := map[string]int{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if cookie, err := r.Cookie("authara_access"); err != nil || cookie.Value != "session" {
			t.Errorf("access cookie = %v, %v", cookie, err)
		}
		if _, ok := wantedPaths[r.URL.Path]; !ok {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		seenPaths[r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/auth/api/v1/capabilities":
			_, _ = w.Write([]byte(`{"organization_mode":"multi","has_visible_organizations":true,"allows_invitations":true,"allows_org_switching":true,"allows_user_created_team_orgs":true,"allows_organization_leave":true}`))
		case "/auth/api/v1/users/" + userID + "/memberships":
			_, _ = w.Write([]byte(`{"memberships":[{"organization":{"id":"organization-1","name":"Example","kind":"team"},"membership":{"organization_id":"organization-1","user_id":"user-1","role":"owner"}}]}`))
		case "/auth/api/v1/organizations/" + organizationID:
			_, _ = w.Write([]byte(`{"organization":{"id":"organization-1","name":"Example","kind":"team"}}`))
		case "/auth/api/v1/organizations/" + organizationID + "/members":
			_, _ = w.Write([]byte(`{"members":[{"organization_id":"organization-1","user_id":"user-1","role":"owner"}]}`))
		case "/auth/api/v1/organizations/" + organizationID + "/members/" + userID:
			_, _ = w.Write([]byte(`{"member":{"organization_id":"organization-1","user_id":"user-1","role":"owner"}}`))
		case "/auth/api/v1/organizations/" + organizationID + "/invitations":
			_, _ = w.Write([]byte(`{"invitations":[{"id":"invitation-1","organization_id":"organization-1","email":"invitee@example.com","role":"member","status":"pending"}]}`))
		}
	}))
	defer server.Close()
	t.Setenv("AUTHARA_BASE_URL", server.URL)

	incoming := httptest.NewRequest(http.MethodGet, "/private", nil)
	incoming.AddCookie(&http.Cookie{Name: "authara_access", Value: "session"})

	capabilities, err := getCapabilities(context.Background(), incoming)
	if err != nil {
		t.Fatalf("getCapabilities() error = %v", err)
	}
	if capabilities.OrganizationMode != "multi" {
		t.Errorf("organization mode = %q, want multi", capabilities.OrganizationMode)
	}

	memberships, err := getUserMemberships(context.Background(), incoming, userID)
	if err != nil {
		t.Fatalf("getUserMemberships() error = %v", err)
	}
	live, liveErrors := loadLiveOrganizations(
		context.Background(),
		incoming,
		userID,
		[]organizationDTO{{ID: organizationID, Name: "Example", Role: "owner"}},
		memberships,
	)
	if len(liveErrors) != 0 {
		t.Fatalf("loadLiveOrganizations() errors = %v", liveErrors)
	}
	if len(live) != 1 || len(live[0].Members) != 1 || len(live[0].Invitations) != 1 {
		t.Fatalf("live organizations = %#v", live)
	}

	for path, want := range wantedPaths {
		if got := seenPaths[path]; got != want {
			t.Errorf("requests to %s = %d, want %d", path, got, want)
		}
	}
}

func TestPublicOrganizationMutationsSendCSRF(t *testing.T) {
	const organizationID = "organization-1"
	csrfRequests := 0
	mutationRequests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if access, err := r.Cookie("authara_access"); err != nil || access.Value != "session" {
			t.Errorf("access cookie = %v, %v", access, err)
		}

		switch r.URL.Path {
		case "/auth/api/v1/csrf":
			csrfRequests++
			http.SetCookie(w, &http.Cookie{Name: "authara_csrf", Value: "csrf-cookie"})
			_, _ = w.Write([]byte(`{"csrf_token":"csrf-token"}`))
		case "/auth/api/v1/organizations/" + organizationID:
			mutationRequests++
			assertCSRFRequest(t, r, http.MethodPatch)
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode update body: %v", err)
				return
			}
			if body["name"] != "Renamed" {
				t.Errorf("name = %q, want Renamed", body["name"])
			}
			_, _ = w.Write([]byte(`{"organization":{"id":"organization-1","name":"Renamed","kind":"team"}}`))
		case "/auth/api/v1/organizations/" + organizationID + "/invitations/invitation-1/revoke":
			mutationRequests++
			assertCSRFRequest(t, r, http.MethodPost)
			_, _ = w.Write([]byte(`{"invitation":{"id":"invitation-1","organization_id":"organization-1","email":"invitee@example.com","role":"member","status":"revoked"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("AUTHARA_BASE_URL", server.URL)

	incoming := httptest.NewRequest(http.MethodPost, "/private", nil)
	incoming.AddCookie(&http.Cookie{Name: "authara_access", Value: "session"})

	if _, err := updateOrganization(incoming, organizationID, "Renamed"); err != nil {
		t.Fatalf("updateOrganization() error = %v", err)
	}
	if _, err := revokeInvitation(incoming, organizationID, "invitation-1"); err != nil {
		t.Fatalf("revokeInvitation() error = %v", err)
	}
	if csrfRequests != 2 || mutationRequests != 2 {
		t.Errorf("requests = csrf:%d mutations:%d, want 2 each", csrfRequests, mutationRequests)
	}
}

func assertCSRFRequest(t *testing.T, r *http.Request, wantMethod string) {
	t.Helper()
	if r.Method != wantMethod {
		t.Errorf("method = %s, want %s", r.Method, wantMethod)
	}
	if got := r.Header.Get("X-CSRF-Token"); got != "csrf-token" {
		t.Errorf("X-CSRF-Token = %q, want csrf-token", got)
	}
	if cookie, err := r.Cookie("authara_csrf"); err != nil || cookie.Value != "csrf-cookie" {
		t.Errorf("CSRF cookie = %v, %v", cookie, err)
	}
}
