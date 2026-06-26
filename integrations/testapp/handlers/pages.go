package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/authara-org/authara-go/authara"
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

type orgState struct {
	ID          string
	Name        string
	Members     map[string]memberState
	Invitations map[string]invitationState
}

type memberState struct {
	ID    string
	Email string
	Role  string
}

type invitationState struct {
	ID        string
	Email     string
	Role      string
	Status    string
	ExpiresAt string
	InviteURL string
}

var projection = struct {
	sync.Mutex
	Orgs map[string]*orgState
}{
	// ponytail: in-memory test projection; use a DB if the test app needs restart-safe state.
	Orgs: map[string]*orgState{},
}

func Home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `
		<html>
			<body>
				<h1>TestApp</h1>
				<a href="/auth/login?return_to=/private">Login</a>
			</body>
		</html>
	`)
}

func Private(w http.ResponseWriter, r *http.Request) {
	user, err := getCurrentUser(r.Context(), r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, "internal error")
		return
	}
	if user == nil {
		http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}

	logout, ok := authara.LogoutFormDataFromRequest(
		r,
		"/auth/login?return_to=/private",
	)
	if !ok {
		http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}

	upsertCurrentUser(user)
	publicOrgs, publicOrgsErr := getUserOrganizations(r.Context(), r)
	currentOrg, currentOrgErr := getCurrentOrganization(r.Context(), r)
	capabilities, capabilitiesErr := getCapabilities(r.Context())
	userMemberships, userMembershipsErr := getUserMemberships(r.Context(), user.ID)
	liveOrgs, liveOrgErrors := loadLiveOrganizations(r.Context(), user.ID, publicOrgs, userMemberships)

	notice := r.URL.Query().Get("notice")
	errMsg := r.URL.Query().Get("error")

	fmt.Fprintf(w, `
		<html>
			<body>
				<h1>Private Page</h1>
				<p>You are authenticated.</p>
				<p><strong>Email:</strong> %s</p>
				<p><strong>Username:</strong> %s</p>
				%s
				%s

				<h2>Create organization</h2>
				%s

				<h2>Invite teammate</h2>
				%s

				<h2>Public organization API</h2>
				%s

				<h2>Internal API</h2>
				%s

				<h2>Webhook projection</h2>
				%s

				<form method="%s" action="%s">
					<input type="hidden" name="%s" value="%s">
					<button type="submit">Logout</button>
				</form>
				<a href="/auth/account">Show Account</a>
			</body>
			<script>
				window.addEventListener("pageshow", (event) => {
				  if (event.persisted) {
				    window.location.reload();
				  }
				});
			</script>
		</html>
	`,
		html.EscapeString(user.Email),
		html.EscapeString(user.Username),
		renderNotice(notice),
		renderError(errMsg),
		renderCreateOrganizationForm(),
		renderInviteForm(publicOrgs),
		renderPublicOrganizations(publicOrgs, publicOrgsErr, currentOrg, currentOrgErr),
		renderInternalAPI(capabilities, capabilitiesErr, userMemberships, userMembershipsErr, liveOrgs, liveOrgErrors),
		renderProjectedOrganizations(user.ID),
		html.EscapeString(logout.Method),
		html.EscapeString(logout.Action),
		html.EscapeString(logout.CSRFName),
		html.EscapeString(logout.CSRFValue),
	)
}

func CreateOrganizationPost(w http.ResponseWriter, r *http.Request) {
	user, err := getCurrentUser(r.Context(), r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectPrivateError(w, r, "invalid form")
		return
	}

	org, err := createOrganization(r.Context(), strings.TrimSpace(r.FormValue("name")), user.ID)
	if err != nil {
		redirectPrivateError(w, r, err.Error())
		return
	}

	recordOrganization(org.Organization, org.Membership)
	http.Redirect(w, r, "/private?notice=organization+created", http.StatusSeeOther)
}

func UpdateOrganizationPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectPrivateError(w, r, "invalid form")
		return
	}

	org, err := updateOrganization(r.Context(), strings.TrimSpace(r.FormValue("organization_id")), strings.TrimSpace(r.FormValue("name")))
	if err != nil {
		redirectPrivateError(w, r, err.Error())
		return
	}

	recordOrganization(org.Organization, nil)
	http.Redirect(w, r, "/private?notice=organization+updated", http.StatusSeeOther)
}

func SwitchOrganizationPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectPrivateError(w, r, "invalid form")
		return
	}
	if err := switchOrganization(w, r, strings.TrimSpace(r.FormValue("organization_id"))); err != nil {
		redirectPrivateError(w, r, err.Error())
		return
	}
	http.Redirect(w, r, "/private?notice=organization+switched", http.StatusSeeOther)
}

func InvitePost(w http.ResponseWriter, r *http.Request) {
	user, err := getCurrentUser(r.Context(), r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectPrivateError(w, r, "invalid form")
		return
	}

	orgID := strings.TrimSpace(r.FormValue("organization_id"))
	email := strings.TrimSpace(r.FormValue("email"))
	if orgID == "" || email == "" {
		redirectPrivateError(w, r, "organization and email required")
		return
	}

	inv, err := createInvitation(r.Context(), orgID, user.ID, email)
	if err != nil {
		redirectPrivateError(w, r, err.Error())
		return
	}

	recordInvitation(inv.Invitation)
	http.Redirect(w, r, "/private?notice=invitation+created", http.StatusSeeOther)
}

func RevokeInvitationPost(w http.ResponseWriter, r *http.Request) {
	user, err := getCurrentUser(r.Context(), r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectPrivateError(w, r, "invalid form")
		return
	}

	inv, err := revokeInvitation(
		r.Context(),
		strings.TrimSpace(r.FormValue("organization_id")),
		strings.TrimSpace(r.FormValue("invitation_id")),
		user.ID,
	)
	if err != nil {
		redirectPrivateError(w, r, err.Error())
		return
	}

	recordInternalInvitation(inv.Invitation)
	http.Redirect(w, r, "/private?notice=invitation+revoked", http.StatusSeeOther)
}

func ResendInvitationPost(w http.ResponseWriter, r *http.Request) {
	user, err := getCurrentUser(r.Context(), r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/auth/login?return_to=/private", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectPrivateError(w, r, "invalid form")
		return
	}

	orgID := strings.TrimSpace(r.FormValue("organization_id"))
	invitationID := strings.TrimSpace(r.FormValue("invitation_id"))
	inv, err := resendInvitation(r.Context(), orgID, invitationID)
	if err != nil {
		redirectPrivateError(w, r, err.Error())
		return
	}

	recordInvitationRevoked(orgID, invitationID)
	recordInternalInvitation(inv.Invitation)
	http.Redirect(w, r, "/private?notice=invitation+resent", http.StatusSeeOther)
}

func RecordWebhook(evt *authara.WebhookEvent) error {
	switch evt.Type {
	case "organization.deleted":
		var data struct {
			OrganizationID string `json:"organization_id"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		deleteOrganization(data.OrganizationID)

	case "organization.created", "organization.updated":
		var data struct {
			OrganizationID string `json:"organization_id"`
			Name           string `json:"name"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		recordOrganization(internalOrganizationDTO{ID: data.OrganizationID, Name: data.Name}, nil)

	case "organization.invitation.created":
		var data invitationState
		var raw struct {
			ID             string `json:"invitation_id"`
			OrganizationID string `json:"organization_id"`
			Email          string `json:"email"`
			Role           string `json:"role"`
			ExpiresAt      string `json:"expires_at"`
		}
		if err := json.Unmarshal(evt.Data, &raw); err != nil {
			return err
		}
		data.ID = raw.ID
		data.Email = raw.Email
		data.Role = raw.Role
		data.Status = "pending"
		data.ExpiresAt = raw.ExpiresAt
		recordInvitationForOrg(raw.OrganizationID, data)

	case "organization.invitation.accepted":
		var data struct {
			InvitationID     string `json:"invitation_id"`
			OrganizationID   string `json:"organization_id"`
			Email            string `json:"email"`
			Role             string `json:"role"`
			AcceptedByUserID string `json:"accepted_by_user_id"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		recordInvitationAccepted(data.OrganizationID, data.InvitationID, data.AcceptedByUserID, data.Email, data.Role)

	case "organization.invitation.revoked":
		var data struct {
			InvitationID   string `json:"invitation_id"`
			OrganizationID string `json:"organization_id"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		recordInvitationRevoked(data.OrganizationID, data.InvitationID)

	case "organization.membership.created", "organization.membership.updated":
		var data struct {
			OrganizationID string `json:"organization_id"`
			UserID         string `json:"user_id"`
			Role           string `json:"role"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		recordMember(data.OrganizationID, memberState{ID: data.UserID, Role: data.Role})

	case "organization.membership.deleted":
		var data struct {
			OrganizationID string `json:"organization_id"`
			UserID         string `json:"user_id"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		deleteMember(data.OrganizationID, data.UserID)
	}
	return nil
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
				var one internalInvitationResponse
				if err := internalJSON(ctx, http.MethodGet, "/auth/internal/v1/organizations/"+url.PathEscape(orgID)+"/invitations/"+url.PathEscape(inv.ID), nil, http.StatusOK, &one); err == nil {
					inv = one.Invitation
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

func upsertCurrentUser(user *currentUser) {
	if user.Organization == nil {
		return
	}
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(user.Organization.ID, user.Organization.Name)
	org.Members[user.ID] = memberState{
		ID:    user.ID,
		Email: user.Email,
		Role:  user.Organization.Role,
	}
}

func recordInvitation(inv invitationDTO) {
	recordInvitationForOrg(inv.OrganizationID, invitationState{
		ID:        inv.ID,
		Email:     inv.Email,
		Role:      inv.Role,
		Status:    inv.Status,
		ExpiresAt: inv.ExpiresAt,
		InviteURL: inv.InviteURL,
	})
}

func recordInternalInvitation(inv internalInvitationDTO) {
	recordInvitationForOrg(inv.OrganizationID, invitationState{
		ID:        inv.ID,
		Email:     inv.Email,
		Role:      inv.Role,
		Status:    inv.Status,
		ExpiresAt: inv.ExpiresAt,
		InviteURL: inv.InviteURL,
	})
}

func recordOrganization(org internalOrganizationDTO, member *internalMembershipDTO) {
	projection.Lock()
	defer projection.Unlock()

	state := ensureOrgLocked(org.ID, org.Name)
	if member != nil && member.UserID != "" {
		state.Members[member.UserID] = memberState{ID: member.UserID, Role: member.Role}
	}
}

func deleteOrganization(orgID string) {
	projection.Lock()
	defer projection.Unlock()

	delete(projection.Orgs, orgID)
}

func recordInvitationForOrg(orgID string, inv invitationState) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	if inv.Status == "" {
		inv.Status = "pending"
	}
	org.Invitations[inv.ID] = inv
}

func recordInvitationAccepted(orgID, invitationID, userID, email, role string) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	inv := org.Invitations[invitationID]
	inv.Status = "accepted"
	org.Invitations[invitationID] = inv
	org.Members[userID] = memberState{ID: userID, Email: email, Role: role}
}

func recordInvitationRevoked(orgID, invitationID string) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	inv := org.Invitations[invitationID]
	inv.Status = "revoked"
	org.Invitations[invitationID] = inv
}

func recordMember(orgID string, member memberState) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	current := org.Members[member.ID]
	if member.Email == "" {
		member.Email = current.Email
	}
	org.Members[member.ID] = member
}

func deleteMember(orgID, userID string) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	delete(org.Members, userID)
}

func ensureOrgLocked(id, name string) *orgState {
	org := projection.Orgs[id]
	if org == nil {
		org = &orgState{
			ID:          id,
			Members:     map[string]memberState{},
			Invitations: map[string]invitationState{},
		}
		projection.Orgs[id] = org
	}
	if name != "" {
		org.Name = name
	}
	if org.Name == "" {
		org.Name = id
	}
	return org
}

func renderCreateOrganizationForm() string {
	return `<form method="post" action="/private/organizations">
		<input name="name" placeholder="New organization name" required>
		<button type="submit">Create organization</button>
	</form>`
}

func renderInviteForm(orgs []organizationDTO) string {
	var b strings.Builder
	b.WriteString(`<form method="post" action="/private/invitations"><select name="organization_id">`)
	for _, org := range orgs {
		fmt.Fprintf(&b, `<option value="%s">%s</option>`, html.EscapeString(org.ID), html.EscapeString(org.Name))
	}
	if len(orgs) == 0 {
		return `<p>No organization available.</p>`
	}
	b.WriteString(`</select> <input type="email" name="email" placeholder="teammate@example.com" required> `)
	b.WriteString(`<button type="submit">Invite member</button></form>`)
	return b.String()
}

func renderPublicOrganizations(orgs []organizationDTO, orgsErr error, current *currentOrg, currentErr error) string {
	var b strings.Builder
	if currentErr != nil {
		fmt.Fprintf(&b, `<p style="color: red;">Current org error: %s</p>`, html.EscapeString(currentErr.Error()))
	} else if current != nil {
		fmt.Fprintf(&b, `<p><strong>Current:</strong> %s <code>%s</code> <small>%s</small></p>`, html.EscapeString(current.Name), html.EscapeString(current.ID), html.EscapeString(current.Role))
	}
	if orgsErr != nil {
		fmt.Fprintf(&b, `<p style="color: red;">Organizations error: %s</p>`, html.EscapeString(orgsErr.Error()))
		return b.String()
	}
	if len(orgs) == 0 {
		b.WriteString(`<p>No organizations returned by public API.</p>`)
		return b.String()
	}

	b.WriteString(`<ul>`)
	for _, org := range orgs {
		fmt.Fprintf(&b, `<li>%s <code>%s</code> <small>%s</small>`, html.EscapeString(org.Name), html.EscapeString(org.ID), html.EscapeString(org.Role))
		fmt.Fprintf(&b, `<form method="post" action="/private/organizations/switch" style="display:inline">
			<input type="hidden" name="organization_id" value="%s">
			<button type="submit">Switch</button>
		</form>`, html.EscapeString(org.ID))
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ul>`)
	return b.String()
}

func renderInternalAPI(
	capabilities *capabilitiesResponse,
	capabilitiesErr error,
	memberships []internalMembershipWithOrganizationDTO,
	membershipsErr error,
	liveOrgs []liveOrganization,
	liveOrgErrors []string,
) string {
	var b strings.Builder
	b.WriteString(`<h3>Capabilities</h3>`)
	if capabilitiesErr != nil {
		fmt.Fprintf(&b, `<p style="color: red;">%s</p>`, html.EscapeString(capabilitiesErr.Error()))
	} else {
		fmt.Fprintf(&b, `<ul>
			<li>mode: <code>%s</code></li>
			<li>visible orgs: %t</li>
			<li>invitations: %t</li>
			<li>switching: %t</li>
			<li>user-created team orgs: %t</li>
			<li>leave org: %t</li>
		</ul>`,
			html.EscapeString(capabilities.OrganizationMode),
			capabilities.HasVisibleOrganizations,
			capabilities.AllowsInvitations,
			capabilities.AllowsOrgSwitching,
			capabilities.AllowsUserCreatedTeamOrgs,
			capabilities.AllowsOrganizationLeave,
		)
	}

	b.WriteString(`<h3>User memberships</h3>`)
	if membershipsErr != nil {
		fmt.Fprintf(&b, `<p style="color: red;">%s</p>`, html.EscapeString(membershipsErr.Error()))
	} else if len(memberships) == 0 {
		b.WriteString(`<p>No memberships returned by internal API.</p>`)
	} else {
		b.WriteString(`<ul>`)
		for _, membership := range memberships {
			fmt.Fprintf(&b, `<li>%s <code>%s</code> <small>%s</small></li>`, html.EscapeString(membership.Organization.Name), html.EscapeString(membership.Organization.ID), html.EscapeString(membership.Membership.Role))
		}
		b.WriteString(`</ul>`)
	}

	if len(liveOrgErrors) > 0 {
		b.WriteString(`<h3>Live organization errors</h3><ul>`)
		for _, msg := range liveOrgErrors {
			fmt.Fprintf(&b, `<li style="color: red;">%s</li>`, html.EscapeString(msg))
		}
		b.WriteString(`</ul>`)
	}

	b.WriteString(`<h3>Live organizations</h3>`)
	if len(liveOrgs) == 0 {
		b.WriteString(`<p>No live organizations loaded.</p>`)
		return b.String()
	}
	for _, org := range liveOrgs {
		fmt.Fprintf(&b, `<section><h4>%s</h4><p><code>%s</code> <small>%s</small></p>`, html.EscapeString(org.Organization.Name), html.EscapeString(org.Organization.ID), html.EscapeString(org.Organization.Kind))
		for _, msg := range org.Errors {
			fmt.Fprintf(&b, `<p style="color: red;">%s</p>`, html.EscapeString(msg))
		}
		fmt.Fprintf(&b, `<form method="post" action="/private/organizations/update">
			<input type="hidden" name="organization_id" value="%s">
			<input name="name" value="%s" required>
			<button type="submit">Rename</button>
		</form>`, html.EscapeString(org.Organization.ID), html.EscapeString(org.Organization.Name))

		if org.CurrentMember != nil {
			fmt.Fprintf(&b, `<p>Your internal member row: <code>%s</code> <small>%s</small></p>`, html.EscapeString(org.CurrentMember.UserID), html.EscapeString(org.CurrentMember.Role))
		}

		b.WriteString(`<h5>Members</h5><ul>`)
		if len(org.Members) == 0 {
			b.WriteString(`<li>None</li>`)
		}
		for _, member := range org.Members {
			fmt.Fprintf(&b, `<li><code>%s</code> <small>%s</small></li>`, html.EscapeString(member.UserID), html.EscapeString(member.Role))
		}
		b.WriteString(`</ul><h5>Invitations</h5><ul>`)
		if len(org.Invitations) == 0 {
			b.WriteString(`<li>None</li>`)
		}
		for _, inv := range org.Invitations {
			fmt.Fprintf(&b, `<li>%s <small>%s, %s, expires %s</small>`, html.EscapeString(inv.Email), html.EscapeString(inv.Role), html.EscapeString(inv.Status), html.EscapeString(inv.ExpiresAt))
			if inv.InviteURL != "" {
				fmt.Fprintf(&b, ` <a href="%s">link</a>`, html.EscapeString(inv.InviteURL))
			}
			if inv.Status == "pending" {
				fmt.Fprintf(&b, ` <form method="post" action="/private/invitations/revoke" style="display:inline">
					<input type="hidden" name="organization_id" value="%s">
					<input type="hidden" name="invitation_id" value="%s">
					<button type="submit">Revoke</button>
				</form>`, html.EscapeString(org.Organization.ID), html.EscapeString(inv.ID))
			}
			if inv.Status == "pending" || inv.Status == "expired" {
				fmt.Fprintf(&b, ` <form method="post" action="/private/invitations/resend" style="display:inline">
					<input type="hidden" name="organization_id" value="%s">
					<input type="hidden" name="invitation_id" value="%s">
					<button type="submit">Resend</button>
				</form>`, html.EscapeString(org.Organization.ID), html.EscapeString(inv.ID))
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ul></section>`)
	}
	return b.String()
}

func renderProjectedOrganizations(userID string) string {
	projection.Lock()
	defer projection.Unlock()

	var b strings.Builder
	count := 0
	for _, org := range projection.Orgs {
		if _, ok := org.Members[userID]; !ok {
			continue
		}
		count++
		fmt.Fprintf(&b, `<section><h3>%s</h3><p><code>%s</code></p>`, html.EscapeString(org.Name), html.EscapeString(org.ID))
		b.WriteString(`<h4>Members</h4><ul>`)
		for _, m := range org.Members {
			label := m.ID
			if m.Email != "" {
				label = m.Email
			}
			fmt.Fprintf(&b, `<li>%s <small>(%s)</small></li>`, html.EscapeString(label), html.EscapeString(m.Role))
		}
		b.WriteString(`</ul><h4>Pending invitations</h4><ul>`)
		pending := 0
		for _, inv := range org.Invitations {
			if inv.Status != "" && inv.Status != "pending" {
				continue
			}
			pending++
			fmt.Fprintf(&b, `<li>%s <small>(%s, expires %s)</small>`, html.EscapeString(inv.Email), html.EscapeString(inv.Role), html.EscapeString(inv.ExpiresAt))
			if inv.InviteURL != "" {
				fmt.Fprintf(&b, ` <a href="%s">link</a>`, html.EscapeString(inv.InviteURL))
			}
			b.WriteString(`</li>`)
		}
		if pending == 0 {
			b.WriteString(`<li>None</li>`)
		}
		b.WriteString(`</ul></section>`)
	}
	if count == 0 {
		return `<p>No organizations projected yet.</p>`
	}
	return b.String()
}

func renderNotice(v string) string {
	if v == "" {
		return ""
	}
	return `<p style="color: green;">` + html.EscapeString(v) + `</p>`
}

func renderError(v string) string {
	if v == "" {
		return ""
	}
	return `<p style="color: red;">` + html.EscapeString(v) + `</p>`
}

func redirectPrivateError(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/private?error="+url.QueryEscape(msg), http.StatusSeeOther)
}
