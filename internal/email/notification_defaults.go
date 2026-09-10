package email

import (
	"fmt"
	"html"
	"strings"
)

type notificationDefaults struct {
	Text string
	HTML string
}

type notificationDetail struct {
	Label string
	Value string
}

var (
	defaultAccountCreated = newNotificationDefaults(
		"Welcome to Authara",
		"Your Authara account was created successfully.",
		[]notificationDetail{
			{Label: "Username", Value: "{{username}}"},
			{Label: "Sign-in method", Value: "{{auth_method}}"},
			{Label: "Created at", Value: "{{occurred_at}}"},
		},
		"If you did not create this account, contact the operator of this Authara deployment.",
		"You received this email because an Authara account was created for your address.",
	)
	defaultNewSignIn = newNotificationDefaults(
		"New sign-in",
		"A new session was created for your Authara account.",
		[]notificationDetail{
			{Label: "IP address", Value: "{{ip_address}}"},
			{Label: "Device", Value: "{{user_agent}}"},
			{Label: "Time", Value: "{{occurred_at}}"},
		},
		"If this was not you, revoke the session and secure your account immediately.",
		"You received this email because a new session was created for your Authara account.",
	)
	defaultAuthMethodAdded = newNotificationDefaults(
		"Sign-in method added",
		"A new sign-in method was added to your Authara account.",
		[]notificationDetail{
			{Label: "Method", Value: "{{auth_method}}"},
			{Label: "Time", Value: "{{occurred_at}}"},
		},
		"If you did not make this change, secure your account immediately.",
		"You received this email because your Authara sign-in methods changed.",
	)
	defaultAuthMethodRemoved = newNotificationDefaults(
		"Sign-in method removed",
		"A sign-in method was removed from your Authara account.",
		[]notificationDetail{
			{Label: "Method", Value: "{{auth_method}}"},
			{Label: "Time", Value: "{{occurred_at}}"},
		},
		"If you did not make this change, secure your account immediately.",
		"You received this email because your Authara sign-in methods changed.",
	)
	defaultPasswordChanged = newNotificationDefaults(
		"Password changed",
		"The password for your Authara account was changed successfully.",
		[]notificationDetail{{Label: "Time", Value: "{{occurred_at}}"}},
		"If you did not change your password, secure your account immediately.",
		"You received this email because the password for your Authara account changed.",
	)
	defaultEmailChangedOldAddress = newNotificationDefaults(
		"Email address changed",
		"The email address for your Authara account was changed.",
		[]notificationDetail{
			{Label: "Previous address", Value: "{{old_email}}"},
			{Label: "New address", Value: "{{new_email}}"},
			{Label: "Time", Value: "{{occurred_at}}"},
		},
		"If you did not make this change, contact the operator of this Authara deployment immediately.",
		"This security notice was sent to the previous email address on the account.",
	)
	defaultEmailChangedNewAddress = newNotificationDefaults(
		"Email address confirmed",
		"This address is now connected to your Authara account.",
		[]notificationDetail{
			{Label: "Previous address", Value: "{{old_email}}"},
			{Label: "New address", Value: "{{new_email}}"},
			{Label: "Time", Value: "{{occurred_at}}"},
		},
		"No further action is required.",
		"You received this email because your Authara account email address changed.",
	)
	defaultAccountDisabled = newNotificationDefaults(
		"Account disabled",
		"An administrator disabled your Authara account and revoked its active sessions.",
		[]notificationDetail{{Label: "Time", Value: "{{occurred_at}}"}},
		"Contact the operator of this Authara deployment if you believe this was a mistake.",
		"You received this email because access to your Authara account changed.",
	)
	defaultAccountEnabled = newNotificationDefaults(
		"Account enabled",
		"An administrator enabled your Authara account.",
		[]notificationDetail{{Label: "Time", Value: "{{occurred_at}}"}},
		"You can sign in again using one of your configured sign-in methods.",
		"You received this email because access to your Authara account changed.",
	)
	defaultAdminAccessChanged = newNotificationDefaults(
		"Administrator access changed",
		"Your administrator access in Authara was {{access_change}}.",
		[]notificationDetail{
			{Label: "Change", Value: "{{access_change}}"},
			{Label: "Time", Value: "{{occurred_at}}"},
		},
		"Contact the operator of this Authara deployment if you did not expect this change.",
		"You received this email because your Authara permissions changed.",
	)
	defaultOrganizationInvitationAccepted = newNotificationDefaults(
		"Organization invitation accepted",
		"An invitation to {{organization_name}} was accepted.",
		[]notificationDetail{
			{Label: "Member", Value: "{{member_email}}"},
			{Label: "Role", Value: "{{role}}"},
			{Label: "Accepted at", Value: "{{occurred_at}}"},
		},
		"The member can now access the organization according to their assigned role.",
		"You received this email because you invited this member to an Authara organization.",
	)
	defaultOrganizationInvitationRevoked = newNotificationDefaults(
		"Organization invitation revoked",
		"Your invitation to {{organization_name}} was revoked.",
		[]notificationDetail{
			{Label: "Organization", Value: "{{organization_name}}"},
			{Label: "Revoked at", Value: "{{occurred_at}}"},
		},
		"The previous invitation link and code can no longer be used.",
		"You received this email because an Authara organization invitation for your address changed.",
	)
	defaultOrganizationMembershipRemoved = newNotificationDefaults(
		"Organization access removed",
		"Your access to {{organization_name}} was removed.",
		[]notificationDetail{
			{Label: "Organization", Value: "{{organization_name}}"},
			{Label: "Previous role", Value: "{{role}}"},
			{Label: "Removed at", Value: "{{occurred_at}}"},
		},
		"Sessions scoped to this organization have been revoked.",
		"You received this email because your Authara organization membership changed.",
	)
	defaultOrganizationRoleChanged = newNotificationDefaults(
		"Organization role changed",
		"Your role in {{organization_name}} was changed.",
		[]notificationDetail{
			{Label: "Previous role", Value: "{{previous_role}}"},
			{Label: "New role", Value: "{{role}}"},
			{Label: "Changed at", Value: "{{occurred_at}}"},
		},
		"Your permissions now reflect the new role.",
		"You received this email because your Authara organization role changed.",
	)
	defaultOrganizationOwnershipTransferred = newNotificationDefaults(
		"Organization ownership transferred",
		"Ownership of {{organization_name}} was transferred.",
		[]notificationDetail{
			{Label: "Previous owner", Value: "{{previous_owner_email}}"},
			{Label: "New owner", Value: "{{new_owner_email}}"},
			{Label: "Transferred at", Value: "{{occurred_at}}"},
		},
		"The previous owner now has the administrator role.",
		"You received this email because you were involved in an Authara organization ownership transfer.",
	)
	defaultOrganizationDeleted = newNotificationDefaults(
		"Organization deleted",
		"The Authara organization {{organization_name}} was deleted.",
		[]notificationDetail{
			{Label: "Organization", Value: "{{organization_name}}"},
			{Label: "Deleted at", Value: "{{occurred_at}}"},
		},
		"Access and sessions scoped to this organization are no longer available.",
		"You received this email because you were a member of this Authara organization.",
	)
)

func newNotificationDefaults(
	heading string,
	introduction string,
	details []notificationDetail,
	notice string,
	footer string,
) notificationDefaults {
	var textBody strings.Builder
	textBody.WriteString(heading)
	textBody.WriteString("\n\n")
	textBody.WriteString(introduction)
	for _, detail := range details {
		textBody.WriteString("\n")
		textBody.WriteString(detail.Label)
		textBody.WriteString(": ")
		textBody.WriteString(detail.Value)
	}
	textBody.WriteString("\n\n")
	textBody.WriteString(notice)
	textBody.WriteString("\n\nAuthara")

	var rows strings.Builder
	for i, detail := range details {
		border := "border-bottom:1px solid #e5e7eb;"
		if i == len(details)-1 {
			border = ""
		}
		fmt.Fprintf(
			&rows,
			`<tr><td style="padding:14px 18px;text-align:left;%s"><div style="font-size:12px;letter-spacing:0.08em;text-transform:uppercase;color:#6b7280;margin-bottom:5px;">%s</div><div style="font-size:15px;line-height:1.5;font-weight:600;color:#111827;word-break:break-word;">%s</div></td></tr>`,
			border,
			html.EscapeString(detail.Label),
			detail.Value,
		)
	}

	htmlBody := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
</head>
<body style="margin:0;padding:0;background-color:#f5f7fb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#111827;">
  <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color:#f5f7fb;margin:0;padding:24px 0;">
    <tr><td align="center">
      <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="max-width:560px;margin:0 auto;">
        <tr><td style="padding:0 16px;">
          <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color:#ffffff;border:1px solid #e5e7eb;border-radius:24px;overflow:hidden;">
            <tr><td style="padding:40px 32px 32px;text-align:center;">
              <div style="display:inline-block;background-color:#eff6ff;border-radius:9999px;padding:14px;margin-bottom:20px;font-size:20px;">🔐</div>
              <h1 style="margin:0 0 10px;font-size:30px;line-height:1.2;font-weight:700;color:#111827;">%s</h1>
              <p style="margin:0 0 26px;font-size:16px;line-height:1.6;color:#4b5563;">%s</p>
              <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin:0 0 24px;background-color:#f9fafb;border:1px solid #e5e7eb;border-radius:18px;">%s</table>
              <p style="margin:0;font-size:14px;line-height:1.6;color:#6b7280;">%s</p>
            </td></tr>
            <tr><td style="padding:18px 24px;background-color:#f9fafb;border-top:1px solid #e5e7eb;text-align:center;"><p style="margin:0;font-size:12px;line-height:1.5;color:#6b7280;">Authara · Secure authentication infrastructure</p></td></tr>
          </table>
          <p style="margin:16px 0 0;text-align:center;font-size:12px;line-height:1.5;color:#9ca3af;">%s</p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`,
		html.EscapeString(heading),
		heading,
		introduction,
		rows.String(),
		notice,
		footer,
	)

	return notificationDefaults{Text: textBody.String(), HTML: htmlBody}
}
