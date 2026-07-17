package handlers

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/authara-org/authara-go/authara"
)

func Home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `
		<html>
			<body>
				<h1>SSR App</h1>
				<a href="/auth/login?return_to=/private">Login</a>
				<a href="/auth/signup?return_to=/private">Sign up</a>
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

func redirectPrivateError(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/private?error="+url.QueryEscape(msg), http.StatusSeeOther)
}
