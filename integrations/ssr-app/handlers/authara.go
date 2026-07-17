package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type currentUser struct {
	ID           string      `json:"id"`
	Email        string      `json:"email"`
	Username     string      `json:"username"`
	Organization *currentOrg `json:"organization"`
}

type currentOrg struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type organizationDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type organizationsAPIResponse struct {
	Organizations []organizationDTO `json:"organizations"`
}

type capabilitiesResponse struct {
	OrganizationMode          string `json:"organization_mode"`
	HasVisibleOrganizations   bool   `json:"has_visible_organizations"`
	AllowsInvitations         bool   `json:"allows_invitations"`
	AllowsOrgSwitching        bool   `json:"allows_org_switching"`
	AllowsUserCreatedTeamOrgs bool   `json:"allows_user_created_team_orgs"`
	AllowsOrganizationLeave   bool   `json:"allows_organization_leave"`
}

type internalOrganizationDTO struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	CreatedByUserID string `json:"created_by_user_id"`
}

type internalMembershipDTO struct {
	OrganizationID string `json:"organization_id"`
	UserID         string `json:"user_id"`
	Role           string `json:"role"`
}

type internalMembershipWithOrganizationDTO struct {
	Organization internalOrganizationDTO `json:"organization"`
	Membership   internalMembershipDTO   `json:"membership"`
}

type internalInvitationDTO struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	Status         string `json:"status"`
	ExpiresAt      string `json:"expires_at"`
	InviteURL      string `json:"invite_url"`
}

type liveOrganization struct {
	Organization  internalOrganizationDTO
	CurrentMember *internalMembershipDTO
	Members       []internalMembershipDTO
	Invitations   []internalInvitationDTO
	Errors        []string
}

type createInvitationResponse struct {
	Invitation invitationDTO `json:"invitation"`
}

type internalOrganizationResponse struct {
	Organization internalOrganizationDTO `json:"organization"`
	Membership   *internalMembershipDTO  `json:"membership"`
}

type internalMemberResponse struct {
	Member internalMembershipDTO `json:"member"`
}

type internalMembersResponse struct {
	Members []internalMembershipDTO `json:"members"`
}

type internalInvitationsResponse struct {
	Invitations []internalInvitationDTO `json:"invitations"`
}

type internalInvitationResponse struct {
	Invitation internalInvitationDTO `json:"invitation"`
}

type internalUserMembershipsResponse struct {
	Memberships []internalMembershipWithOrganizationDTO `json:"memberships"`
}

type invitationDTO struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	Status         string `json:"status"`
	ExpiresAt      string `json:"expires_at"`
	InviteURL      string `json:"invite_url"`
}

func getCurrentUser(ctx context.Context, incoming *http.Request) (*currentUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, autharaBaseURL()+"/auth/api/v1/user", nil)
	if err != nil {
		return nil, err
	}
	for _, c := range incoming.Cookies() {
		req.AddCookie(c)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authara user endpoint returned %d", resp.StatusCode)
	}

	var user currentUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func getUserOrganizations(ctx context.Context, incoming *http.Request) ([]organizationDTO, error) {
	var out organizationsAPIResponse
	if err := autharaAPIJSON(ctx, incoming, http.MethodGet, "/auth/api/v1/organizations", nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return out.Organizations, nil
}

func getCurrentOrganization(ctx context.Context, incoming *http.Request) (*currentOrg, error) {
	var out currentOrg
	if err := autharaAPIJSON(ctx, incoming, http.MethodGet, "/auth/api/v1/organizations/current", nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func switchOrganization(w http.ResponseWriter, incoming *http.Request, orgID string) error {
	if orgID == "" {
		return fmt.Errorf("organization required")
	}

	csrfToken, csrfCookies, err := fetchAPICSRF(incoming.Context(), incoming)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		incoming.Context(),
		http.MethodPost,
		autharaBaseURL()+"/auth/api/v1/organizations/"+url.PathEscape(orgID)+"/switch",
		nil,
	)
	if err != nil {
		return err
	}
	addCookies(req, incoming.Cookies(), csrfCookies)
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return autharaResponseError(resp, "organization switch failed")
	}
	for _, raw := range resp.Header.Values("Set-Cookie") {
		w.Header().Add("Set-Cookie", raw)
	}
	return nil
}

func fetchAPICSRF(ctx context.Context, incoming *http.Request) (string, []*http.Cookie, error) {
	var out struct {
		Token string `json:"csrf_token"`
	}
	resp, err := autharaAPIResponse(ctx, incoming, http.MethodGet, "/auth/api/v1/csrf", nil)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, autharaResponseError(resp, "csrf fetch failed")
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", nil, err
	}
	if out.Token == "" {
		return "", nil, fmt.Errorf("csrf fetch failed: empty token")
	}
	return out.Token, resp.Cookies(), nil
}

func autharaAPIJSON(ctx context.Context, incoming *http.Request, method, path string, body any, wantStatus int, out any) error {
	resp, err := autharaAPIResponse(ctx, incoming, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		return autharaResponseError(resp, path+" failed")
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func autharaAPIResponse(ctx context.Context, incoming *http.Request, method, path string, body any) (*http.Response, error) {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, autharaBaseURL()+path, reader)
	if err != nil {
		return nil, err
	}
	addCookies(req, incoming.Cookies(), nil)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

func createInvitation(ctx context.Context, orgID, actorID, email string) (*createInvitationResponse, error) {
	var out createInvitationResponse
	err := internalJSON(
		ctx,
		http.MethodPost,
		"/auth/internal/v1/organizations/"+url.PathEscape(orgID)+"/invitations",
		map[string]string{
			"actor_user_id": actorID,
			"email":         email,
		},
		http.StatusCreated,
		&out,
	)
	return &out, err
}

func createOrganization(ctx context.Context, name, createdByUserID string) (*internalOrganizationResponse, error) {
	if name == "" {
		return nil, fmt.Errorf("organization name required")
	}
	var out internalOrganizationResponse
	err := internalJSON(
		ctx,
		http.MethodPost,
		"/auth/internal/v1/organizations",
		map[string]string{"name": name, "created_by_user_id": createdByUserID},
		http.StatusCreated,
		&out,
	)
	return &out, err
}

func updateOrganization(ctx context.Context, orgID, name string) (*internalOrganizationResponse, error) {
	if orgID == "" || name == "" {
		return nil, fmt.Errorf("organization and name required")
	}
	var out internalOrganizationResponse
	err := internalJSON(
		ctx,
		http.MethodPatch,
		"/auth/internal/v1/organizations/"+url.PathEscape(orgID),
		map[string]string{"name": name},
		http.StatusOK,
		&out,
	)
	return &out, err
}

func revokeInvitation(ctx context.Context, orgID, invitationID, revokedByUserID string) (*internalInvitationResponse, error) {
	if orgID == "" || invitationID == "" {
		return nil, fmt.Errorf("organization and invitation required")
	}
	var out internalInvitationResponse
	err := internalJSON(
		ctx,
		http.MethodPost,
		"/auth/internal/v1/organizations/"+url.PathEscape(orgID)+"/invitations/"+url.PathEscape(invitationID)+"/revoke",
		map[string]string{"revoked_by_user_id": revokedByUserID},
		http.StatusOK,
		&out,
	)
	return &out, err
}

func resendInvitation(ctx context.Context, orgID, invitationID string) (*internalInvitationResponse, error) {
	if orgID == "" || invitationID == "" {
		return nil, fmt.Errorf("organization and invitation required")
	}
	var out internalInvitationResponse
	err := internalJSON(
		ctx,
		http.MethodPost,
		"/auth/internal/v1/organizations/"+url.PathEscape(orgID)+"/invitations/"+url.PathEscape(invitationID)+"/resend",
		nil,
		http.StatusCreated,
		&out,
	)
	return &out, err
}

func getCapabilities(ctx context.Context) (*capabilitiesResponse, error) {
	var out capabilitiesResponse
	if err := internalJSON(ctx, http.MethodGet, "/auth/internal/v1/capabilities", nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func getUserMemberships(ctx context.Context, userID string) ([]internalMembershipWithOrganizationDTO, error) {
	var out internalUserMembershipsResponse
	if err := internalJSON(ctx, http.MethodGet, "/auth/internal/v1/users/"+url.PathEscape(userID)+"/memberships", nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return out.Memberships, nil
}

func loadLiveOrganizations(ctx context.Context, userID string, publicOrgs []organizationDTO, memberships []internalMembershipWithOrganizationDTO) ([]liveOrganization, []string) {
	orgIDs := orgIDsForLiveView(publicOrgs, memberships)
	out := make([]liveOrganization, 0, len(orgIDs))
	var errs []string

	for _, orgID := range orgIDs {
		live := liveOrganization{}
		if err := internalJSON(ctx, http.MethodGet, "/auth/internal/v1/organizations/"+url.PathEscape(orgID), nil, http.StatusOK, &struct {
			Organization *internalOrganizationDTO `json:"organization"`
		}{Organization: &live.Organization}); err != nil {
			live.Errors = append(live.Errors, err.Error())
			errs = append(errs, orgID+": "+err.Error())
		}

		var members internalMembersResponse
		if err := internalJSON(ctx, http.MethodGet, "/auth/internal/v1/organizations/"+url.PathEscape(orgID)+"/members", nil, http.StatusOK, &members); err != nil {
			live.Errors = append(live.Errors, err.Error())
		} else {
			live.Members = members.Members
		}

		var member internalMemberResponse
		if err := internalJSON(ctx, http.MethodGet, "/auth/internal/v1/organizations/"+url.PathEscape(orgID)+"/members/"+url.PathEscape(userID), nil, http.StatusOK, &member); err == nil {
			live.CurrentMember = &member.Member
		}

		var invitations internalInvitationsResponse
		if err := internalJSON(ctx, http.MethodGet, "/auth/internal/v1/organizations/"+url.PathEscape(orgID)+"/invitations", nil, http.StatusOK, &invitations); err != nil {
			live.Errors = append(live.Errors, err.Error())
		} else {
			for _, inv := range invitations.Invitations {
				if inv.Status != "pending" {
					continue
				}
				var one internalInvitationResponse
				if err := internalJSON(ctx, http.MethodGet, "/auth/internal/v1/organizations/"+url.PathEscape(orgID)+"/invitations/"+url.PathEscape(inv.ID), nil, http.StatusOK, &one); err == nil {
					inv = one.Invitation
				}
				if inv.Status != "pending" {
					continue
				}
				live.Invitations = append(live.Invitations, inv)
			}
		}
		if live.Organization.ID == "" {
			live.Organization.ID = orgID
			live.Organization.Name = orgID
		}
		out = append(out, live)
	}
	return out, errs
}

func internalJSON(ctx context.Context, method, path string, body any, wantStatus int, out any) error {
	token := strings.TrimSpace(os.Getenv("AUTHARA_INTERNAL_API_TOKEN"))
	if token == "" {
		return fmt.Errorf("AUTHARA_INTERNAL_API_TOKEN missing")
	}

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, autharaBaseURL()+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		return autharaResponseError(resp, path+" failed")
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func autharaResponseError(resp *http.Response, fallback string) error {
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if env.Error.Message != "" {
		return fmt.Errorf("%s", env.Error.Message)
	}
	if env.Error.Code != "" {
		return fmt.Errorf("%s: %s", fallback, env.Error.Code)
	}
	return fmt.Errorf("%s: %d", fallback, resp.StatusCode)
}

func addCookies(req *http.Request, primary []*http.Cookie, override []*http.Cookie) {
	byName := map[string]*http.Cookie{}
	for _, cookie := range primary {
		byName[cookie.Name] = cookie
	}
	for _, cookie := range override {
		byName[cookie.Name] = cookie
	}
	for _, cookie := range byName {
		req.AddCookie(cookie)
	}
}

func orgIDsForLiveView(publicOrgs []organizationDTO, memberships []internalMembershipWithOrganizationDTO) []string {
	seen := map[string]bool{}
	var out []string
	for _, org := range publicOrgs {
		if org.ID != "" && !seen[org.ID] {
			seen[org.ID] = true
			out = append(out, org.ID)
		}
	}
	for _, membership := range memberships {
		id := membership.Organization.ID
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func autharaBaseURL() string {
	if v := strings.TrimRight(os.Getenv("AUTHARA_BASE_URL"), "/"); v != "" {
		return v
	}
	return "http://authara:8080"
}
