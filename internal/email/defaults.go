package email

import "strings"

const (
	defaultSignupCodeText = `Verify your email

Your verification code is: {{code}}

This code expires after a short time.
If you did not request this email, you can safely ignore it.

Authara`

	defaultPasswordResetCodeText = `Reset your password

Your password reset code is: {{code}}

This code expires after a short time.
If you did not request a password reset, you can safely ignore this email.

Authara`

	defaultEmailChangeCodeText = `Change your email address

Your email change verification code is: {{code}}

This code expires after a short time.
If you did not request this email change, you can safely ignore this email.

Authara`

	defaultOrganizationInvitationText = `You're invited to {{organization_name}}

Accept the invitation: {{invite_url}}

Role: {{role}}
Invitation code: {{invitation_code}}
Expires at: {{expires_at}}

If you did not expect this invitation, you can ignore this email.

Authara`
)

var (
	defaultSignupCodeHTML = defaultCodeHTML(
		"Your verification code",
		"✉️",
		"Verify your email",
		"Use the verification code below to continue signing in to Authara.",
		"Verification code",
		"This code expires after a short time. If you did not request it, you can safely ignore this email.",
		"You received this email because a verification request was started for your address.",
	)
	defaultPasswordResetCodeHTML = defaultCodeHTML(
		"Your password reset code",
		"🔐",
		"Reset your password",
		"Use the verification code below to continue resetting your Authara password.",
		"Password reset code",
		"This code expires after a short time. If you did not request a password reset, you can safely ignore this email.",
		"You received this email because a password reset request was started for your address.",
	)
	defaultEmailChangeCodeHTML = defaultCodeHTML(
		"Your email change code",
		"✉️",
		"Verify your new email",
		"Use the verification code below to confirm your new email address for your Authara account.",
		"Email change code",
		"This code expires after a short time. If you did not request this email change, you can safely ignore this email.",
		"You received this email because an email change request was started for your account.",
	)
)

const defaultCodeHTMLLayout = `<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>__DOCUMENT_TITLE__</title>
</head>
<body style="margin:0;padding:0;background-color:#f5f7fb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#111827;">
  <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="background-color:#f5f7fb;margin:0;padding:24px 0;">
    <tr>
      <td align="center">
        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="max-width:560px;margin:0 auto;">
          <tr>
            <td style="padding:0 16px;">
              <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="background-color:#ffffff;border:1px solid #e5e7eb;border-radius:24px;overflow:hidden;">
                <tr>
                  <td style="padding:40px 32px 32px 32px;text-align:center;">
                    <div style="display:inline-block;background-color:#eff6ff;border-radius:9999px;padding:14px;margin-bottom:20px;">
                      <div style="width:24px;height:24px;line-height:24px;font-size:18px;color:#2563eb;">__ICON__</div>
                    </div>
                    <h1 style="margin:0 0 10px 0;font-size:30px;line-height:1.2;font-weight:700;color:#111827;">__HEADING__</h1>
                    <p style="margin:0 0 28px 0;font-size:16px;line-height:1.6;color:#4b5563;">__INTRODUCTION__</p>
                    <div style="margin:0 auto 28px auto;max-width:320px;background-color:#2563eb;border:2px solid #2563eb;border-radius:18px;padding:18px 20px;">
                      <div style="font-size:12px;letter-spacing:0.08em;text-transform:uppercase;color:#bfdbfe;margin-bottom:8px;">__CODE_LABEL__</div>
                      <div style="font-size:34px;line-height:1;font-weight:700;letter-spacing:0.28em;color:#ffffff;font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,'Courier New',monospace;">{{code}}</div>
                    </div>
                    <p style="margin:0 0 24px 0;font-size:14px;line-height:1.6;color:#6b7280;">__NOTICE__</p>
                  </td>
                </tr>
                <tr>
                  <td style="padding:18px 24px;background-color:#f9fafb;border-top:1px solid #e5e7eb;text-align:center;">
                    <p style="margin:0;font-size:12px;line-height:1.5;color:#6b7280;">Authara · Secure authentication infrastructure</p>
                  </td>
                </tr>
              </table>
              <p style="margin:16px 0 0 0;text-align:center;font-size:12px;line-height:1.5;color:#9ca3af;">__FOOTER__</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

const defaultOrganizationInvitationHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Organization invitation</title>
</head>
<body style="margin:0;padding:0;background-color:#f5f7fb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#111827;">
  <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="background-color:#f5f7fb;margin:0;padding:24px 0;">
    <tr>
      <td align="center">
        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="max-width:560px;margin:0 auto;">
          <tr>
            <td style="padding:0 16px;">
              <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="background-color:#ffffff;border:1px solid #e5e7eb;border-radius:24px;overflow:hidden;">
                <tr>
                  <td style="padding:40px 32px 32px 32px;text-align:center;">
                    <div style="display:inline-block;background-color:#eff6ff;border-radius:9999px;padding:14px;margin-bottom:20px;">
                      <div style="width:24px;height:24px;line-height:24px;font-size:18px;color:#2563eb;">👥</div>
                    </div>
                    <h1 style="margin:0 0 10px 0;font-size:30px;line-height:1.2;font-weight:700;color:#111827;">You're invited to {{organization_name}}</h1>
                    <p style="margin:0 0 28px 0;font-size:16px;line-height:1.6;color:#4b5563;">Accept your invitation to join this organization in Authara.</p>
                    <div style="margin:0 auto 28px auto;max-width:320px;">
                      <a href="{{invite_url}}" style="display:block;background-color:#2563eb;border:2px solid #2563eb;border-radius:18px;padding:18px 20px;font-size:16px;line-height:1.2;font-weight:700;color:#ffffff;text-decoration:none;">Accept invitation</a>
                    </div>
                    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin:0 0 24px 0;background-color:#f9fafb;border:1px solid #e5e7eb;border-radius:18px;">
                      <tr>
                        <td style="padding:16px 18px;text-align:left;border-bottom:1px solid #e5e7eb;">
                          <div style="font-size:12px;letter-spacing:0.08em;text-transform:uppercase;color:#6b7280;margin-bottom:6px;">Role</div>
                          <div style="font-size:16px;line-height:1.4;font-weight:600;color:#111827;">{{role}}</div>
                        </td>
                      </tr>
                      <tr>
                        <td style="padding:16px 18px;text-align:left;border-bottom:1px solid #e5e7eb;">
                          <div style="font-size:12px;letter-spacing:0.08em;text-transform:uppercase;color:#6b7280;margin-bottom:6px;">Invitation code</div>
                          <div style="font-size:16px;line-height:1.4;font-weight:600;color:#111827;font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,'Courier New',monospace;word-break:break-all;">{{invitation_code}}</div>
                        </td>
                      </tr>
                      <tr>
                        <td style="padding:16px 18px;text-align:left;">
                          <div style="font-size:12px;letter-spacing:0.08em;text-transform:uppercase;color:#6b7280;margin-bottom:6px;">Expires at</div>
                          <div style="font-size:16px;line-height:1.4;font-weight:600;color:#111827;">{{expires_at}}</div>
                        </td>
                      </tr>
                    </table>
                    <p style="margin:0 0 24px 0;font-size:14px;line-height:1.6;color:#6b7280;">If you did not expect this invitation, you can ignore this email.</p>
                  </td>
                </tr>
                <tr>
                  <td style="padding:18px 24px;background-color:#f9fafb;border-top:1px solid #e5e7eb;text-align:center;">
                    <p style="margin:0;font-size:12px;line-height:1.5;color:#6b7280;">Authara · Secure authentication infrastructure</p>
                  </td>
                </tr>
              </table>
              <p style="margin:16px 0 0 0;text-align:center;font-size:12px;line-height:1.5;color:#9ca3af;">You received this email because someone invited you to an organization in Authara.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

func defaultCodeHTML(documentTitle, icon, heading, introduction, codeLabel, notice, footer string) string {
	return strings.NewReplacer(
		"__DOCUMENT_TITLE__", documentTitle,
		"__ICON__", icon,
		"__HEADING__", heading,
		"__INTRODUCTION__", introduction,
		"__CODE_LABEL__", codeLabel,
		"__NOTICE__", notice,
		"__FOOTER__", footer,
	).Replace(defaultCodeHTMLLayout)
}
