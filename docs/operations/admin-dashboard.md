# Admin Dashboard

Authara includes an internal admin dashboard at:

```text
/auth/admin
```

The dashboard is a server-rendered templ and HTMX interface. It is intended for operators of a self-hosted Authara deployment and is currently treated as an internal/unstable browser surface, not a stable public API contract.

## Access

Admin pages require an authenticated admin-audience session and the `admin` platform role.

State-changing admin actions use `POST` routes and require the normal Authara CSRF token. Browser form submissions include the token as `csrf_token`.

## Bootstrap The First Admin

Create a user through the normal signup or user provisioning flow, then grant the built-in `admin` platform role:

```sh
make admin-by-email EMAIL=user@example.com
```

The helper preserves the existing user and role rows and inserts into `user_platform_roles` with `ON CONFLICT DO NOTHING`.

## Available Actions

The dashboard supports:

- dashboard counts for total users, signups in the last 24 hours, disabled users, and active sessions
- user lookup by email or username
- user detail inspection for roles, auth providers, passkeys, and sessions
- disabling and re-enabling users
- granting and removing the admin role
- revoking one session or all active sessions for a user
- allowlist list, live search, pagination, add, and remove
- recent failed email jobs and risky challenges
- recent admin audit events

Allowlist management is available only when
`AUTHARA_ACCESS_POLICY_ALLOWLIST_ENABLED=true`. When the flag is false, the
allowlist admin UI is hidden and direct allowlist admin routes return `404 Not
Found`.

## Admin Privacy & Security

Admin pages process personal data. Authara shows email, username, user ID, account status, roles, auth provider names, passkey summaries, and session summaries because those fields are necessary for account administration, security support, and incident response.

Technical identifiers are minimized in the UI:

- session IDs are shortened in tables and full IDs are used only in form routes
- user agents are summarized, with full user agent strings behind an explicit technical-details disclosure
- passkey credential IDs, public keys, password hashes, refresh token hashes, verification code hashes, OAuth tokens, and raw provider identifiers are not rendered
- passkey transports are shown only under technical details

The audit log is for security and accountability, not casual monitoring. The default audit table shows timestamps, actions, shortened actor/target user IDs, and masked emails. Personal data and metadata are behind a disclosure. Audit events are personal data; choose retention based on your legal and security requirements.

`AUTHARA_ADMIN_AUDIT_RETENTION_DAYS` controls admin audit retention. The default is `180` days and must be greater than zero. Authara runs a cleanup worker that removes older admin audit events.

## Security Notes

Authara protects against common admin lockout and stale-access mistakes:

- admins cannot disable themselves
- admins cannot remove their own admin role
- disabling the last active admin is rejected
- removing the last active admin role is rejected
- admins cannot revoke all sessions for their own account from the admin user detail page
- disabling a user revokes active sessions and deletes refresh tokens
- removing the admin role revokes that user's active sessions and deletes refresh tokens
- admin mutations are written to `authara.admin_audit_events`
- password hashes, refresh token hashes, verification code hashes, raw passkey public keys, and OAuth tokens are not rendered in admin templates

Run migrations before using the dashboard. The admin audit table is introduced in schema version `11`.
