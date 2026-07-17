package handlers

import (
	"fmt"
	"html"
	"strings"
)

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
		b.WriteString(`</ul><h5>Pending invitations</h5><ul>`)
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
