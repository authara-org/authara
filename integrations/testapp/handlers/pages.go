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

				<h2>Invite teammate</h2>
				%s

				<h2>Your organizations</h2>
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
		renderInviteForm(user.ID),
		renderOrganizations(user.ID),
		html.EscapeString(logout.Method),
		html.EscapeString(logout.Action),
		html.EscapeString(logout.CSRFName),
		html.EscapeString(logout.CSRFValue),
	)
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

func RecordWebhook(evt *authara.WebhookEvent) error {
	switch evt.Type {
	case "organization_invitation.created":
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

	case "organization_invitation.accepted":
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

	case "organization_membership.created":
		var data struct {
			OrganizationID string `json:"organization_id"`
			UserID         string `json:"user_id"`
			Role           string `json:"role"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		recordMember(data.OrganizationID, memberState{ID: data.UserID, Role: data.Role})
	}
	return nil
}

type createInvitationResponse struct {
	Invitation invitationDTO `json:"invitation"`
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

func createInvitation(ctx context.Context, orgID, actorID, email string) (*createInvitationResponse, error) {
	token := strings.TrimSpace(os.Getenv("AUTHARA_INTERNAL_API_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("AUTHARA_INTERNAL_API_TOKEN missing")
	}

	body, _ := json.Marshal(map[string]string{
		"actor_user_id": actorID,
		"email":         email,
	})
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		autharaBaseURL()+"/auth/internal/v1/organizations/"+url.PathEscape(orgID)+"/invitations",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var env struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&env)
		if env.Error.Message != "" {
			return nil, fmt.Errorf("%s", env.Error.Message)
		}
		return nil, fmt.Errorf("invite failed: %d", resp.StatusCode)
	}

	var out createInvitationResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
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

func renderInviteForm(userID string) string {
	projection.Lock()
	defer projection.Unlock()

	var b strings.Builder
	b.WriteString(`<form method="post" action="/private/invitations"><select name="organization_id">`)
	count := 0
	for _, org := range projection.Orgs {
		if _, ok := org.Members[userID]; !ok {
			continue
		}
		count++
		fmt.Fprintf(&b, `<option value="%s">%s</option>`, html.EscapeString(org.ID), html.EscapeString(org.Name))
	}
	if count == 0 {
		return `<p>No organization available.</p>`
	}
	b.WriteString(`</select> <input type="email" name="email" placeholder="teammate@example.com" required> `)
	b.WriteString(`<button type="submit">Invite member</button></form>`)
	return b.String()
}

func renderOrganizations(userID string) string {
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
