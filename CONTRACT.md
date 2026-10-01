# Authara Contract

This document defines the **public compatibility contract** of Authara.

Its purpose is to make upgrades safe for:

- applications integrating Authara
- SDKs built on top of Authara
- operators deploying Authara in production

If behavior described here changes incompatibly, that change is considered **breaking** and must follow the versioning policy below.

---

# 1. Scope

Authara exposes multiple public contracts:

- OpenAPI contract (public/internal JSON routes, schemas, cookies, and errors)
- configuration contract (environment variables)
- webhook contract (event types, payloads, delivery behavior)
- Redis access-token revocation contract (cross-component key templates)

This document defines the rules and guarantees that apply to all public contracts.

Machine-readable contract definitions live in the `contract/` directory.

---

# 2. Versioning Policy

Authara follows semantic versioning for its public contract.

## Patch release (`x.y.Z`)
Patch releases must **not** break existing integrations.

Allowed in a patch release:

- bug fixes
- security fixes
- performance improvements
- internal refactors
- stricter validation only if it fixes clearly invalid behavior and does not break documented valid usage
- additive non-breaking response fields where consumers can safely ignore them

Not allowed in a patch release:

- removing or renaming public routes
- changing supported HTTP methods for public routes
- renaming cookies
- changing required request fields
- changing stable JSON field names
- changing stable error codes
- changing redirect semantics relied upon by apps or SDKs
- changing webhook payload shape or event names

## Minor release (`x.Y.z`)
Minor releases may introduce **additive** features.

Allowed in a minor release:

- new endpoints
- new optional request fields
- new optional response fields
- new cookies only if existing cookies remain unchanged
- new OAuth providers
- new webhook event types
- new non-breaking SDK integration capabilities

Not allowed in a minor release:

- removing or renaming stable public routes
- removing stable fields or cookies
- changing stable semantics incompatibly
- removing webhook event types

## Major release (`X.y.z`)
Major releases may include breaking changes.

All breaking changes must be documented clearly in release notes and migration guides.

---

# 3. Stability Levels

Authara uses the following stability levels:

## Stable
Behavior is part of the public contract and may not break except in a major release.

## Deprecated
Behavior still works, but is scheduled for removal in a future major release.
Deprecated behavior must be documented before removal.

## Internal
Not part of the public contract.
May change at any time without notice.

Unless explicitly stated otherwise, documented public behavior is considered **Stable**.

---

# 4. HTTP Contract

## 4.1 Stable routes

### Authentication UI / actions

- `GET /auth/login`
- `POST /auth/login`
- `GET /auth/signup`
- `POST /auth/signup`
- `GET /auth/verify-email`
- `POST /auth/verify-email`
- `POST /auth/verify-email/complete`

### Session actions

- `POST /auth/sessions/logout`

### OAuth flow endpoints

Any documented OAuth callback endpoint used by browser-based integrations is part of the public contract once released and documented.

## 4.2 Method stability

For stable public endpoints:

- paths must remain unchanged
- HTTP methods must remain unchanged
- behavior must remain compatible

Changing methods or paths is a breaking change.

## 4.3 Status codes

Consumers may rely on the meaning of important status codes.

Precise HTML content is not stable unless explicitly documented.

---

# 5. Redirect Contract

Redirect behavior is part of the public contract.

## Browser redirects
Used for login, signup, logout, and protected flows.

## HTMX redirects
Use of `HX-Redirect` is stable once documented.

---

# 6. Cookie Contract

Stable cookies:

- `authara_access`
- `authara_refresh`
- `authara_csrf`

## Rules

- names must not change
- roles must remain consistent
- breaking security semantics is not allowed

---

# 7. JSON Contract

## Stable fields
Field names must not change or be removed.

## Error envelope

```json
{
  "error": {
    "code": "unauthorized",
    "message": "..."
  }
}
```

## Stable error codes

- `unauthorized`
- `forbidden`
- `invalid_request`
- `not_found`
- `recent_authentication_required`
- `invalid_authentication_challenge`
- `internal_error`

---

# 8. Authentication Semantics

Stable behaviors include:

- access + refresh token model
- session lifecycle
- logout invalidation
- CSRF protection for browser flows

Security-relevant guarantees must not be weakened.

## 8.1 Recent-authentication contract

Sensitive authenticated mutations may return HTTP `428` with
`recent_authentication_required` when the current session's last password,
passkey, or federated proof is older than the configured window. Token refresh
and organization switching do not update that proof time.

Recent-authentication enforcement is enabled by default. It can be disabled
dynamically with `AUTHARA_RECENT_AUTHENTICATION_ENABLED` or its operator
override. When disabled, protected routes still require the otherwise valid
session, authorization, and CSRF checks, but do not issue a step-up challenge.

The `428` response includes a short-lived `authentication_challenge` bound to
the current user and session. Clients complete that challenge through one of
the reauthentication endpoints and then retry the original mutation. A
challenge is single-use; expired, consumed, or session-mismatched challenges
return HTTP `409` with `invalid_authentication_challenge`.

Application backends can apply the same policy before a sensitive server-side
operation by calling `POST /auth/api/v1/reauthenticate/check` with the user's
session and CSRF proof. It returns `204` when the policy is satisfied or the
same `428` challenge envelope when step-up is required.

## 8.2 Access-token revocation contract

Core exposes two startup-selected revocation guarantees. `immediate` requires
Redis, checks revocation state on every protected request, and fails closed
when that state cannot be read. Access-token lifetime is capped at 24 hours so
revocation markers can cover tokens issued before a runtime policy reduction.
`expiry` requires the noop provider and further caps the access-token lifetime
at 10 minutes; logout and authorization changes stop
refresh immediately, but an already-issued access token can remain usable
until its expiry. Production/noop deployments must select `expiry` explicitly.
Every replica must use the same mode.

Core and server-side SDK middleware in immediate mode share the Redis key
templates in `contract/access-token-revocations.json`. An incompatible change
is breaking unless a compatible rollout supports both formats.

## 8.3 Email-verification contract

Users persist the time at which their current email address was verified.
Changing the address clears that proof unless the replacement is completed by
an email challenge or a verified federated identity for the same address.
Public user representations and access tokens expose the boolean
`email_verified`; user responses also expose `email_verified_at` when present.
Challenge-disabled signup and admin-created accounts start unverified.
Challenge-backed signup starts verified, and a federated provider marks the
current address verified only when the provider asserts that exact address as
verified.

`AUTHARA_EMAIL_VERIFICATION_REQUIRED` defaults to `false`. When enabled, it
applies immediately to existing accounts: an authenticated request carrying an
unverified identity revokes the backing session, records immediate access-token
revocation when Redis revocation is configured, and sends browser users to
`/auth/verify-email`. The verification flow can confirm the existing address or
atomically replace and confirm it, then issues a new session. Password recovery
for an unverified address is opaque and does not send a usable reset code while
the policy is enabled.

## 8.4 Passkey clone-warning contract

A newly detected passkey sign-counter anomaly creates a durable security event.
`AUTHARA_PASSKEY_CLONE_RESPONSE` selects `alert`, `restrict`, or
`restrict_and_revoke`; the latter two deny the current authentication and
persistently restrict the passkey, while the last option also revokes all user
sessions using the access-token guarantees in section 8.2.
`AUTHARA_PASSKEY_CLONE_NOTIFY_USER` controls a generic user notice
that never includes credential IDs, public keys, authenticator IDs, or counter
values. Authenticators that legitimately keep both counters at zero do not
trigger this response.

## 8.5 Password recovery contract

Password reset rotates an existing password provider. It does not add password
authentication to an OAuth-only or passkey-only account. Unknown and
passwordless accounts receive the same public accepted response as eligible
accounts, without a usable reset code, to prevent account and authentication
method enumeration.

A passwordless user must authenticate with an existing provider or passkey
before adding a password. Reset completion revalidates the user, current email,
and existing password provider before atomically changing the password,
revoking sessions, invalidating pending resets, queueing the security
notification, and consuming the challenge.

Authenticated password additions, changes, replacements, and removals
invalidate outstanding password-reset requests so an older code cannot gain
authority over a newly created or changed credential.

An authenticated password change preserves the initiating session and
atomically revokes every other session family, deletes its refresh tokens, and
invalidates outstanding password-reset requests. When a shared revocation
mode is `immediate`, already-issued access tokens for those sessions are also
rejected immediately. In `expiry` mode, they remain usable only until the
capped access-token expiry.

## 8.6 Platform roles

Authara exposes two platform roles: `authara:admin` and `authara:operator`.
Admin-audience sessions require the admin role, and operator-audience sessions
require the operator role. Neither role implies the other.

---

# 9. Webhook Contract

If webhooks are configured, the following are part of the public contract:

- webhook event types
- webhook payload envelope
- webhook header names
- signature format
- delivery semantics

Stable webhook guarantees:

- event type names must not change
- payload fields must not be removed or renamed
- headers must remain consistent
- signature format must remain compatible

Additive changes (e.g. new fields or events) are allowed in minor releases.

The machine-readable webhook contract is defined in:

```
contract/webhooks.yaml
```

---

# 10. Configuration Contract

Authara exposes configuration via environment variables.

Stable configuration variables are part of the public contract and follow the same versioning rules.

The machine-readable configuration contract is defined in:

```
contract/config.yaml
```

---

# 11. What Is Not Stable

Not part of the contract:

- internal code structure
- database schema
- HTML/CSS details
- internal APIs
- logging format
- internal error messages

---

# 12. Compatibility Testing

Authara maintains contract tests for:

- routes and methods
- cookies
- JSON structure and error codes
- redirect behavior
- configuration surface
- webhook event types and payload shape
- webhook signature format
- Redis access-token revocation keys

---

# 13. Release Gate

Before releasing:

> Could an existing integration break without changes?

If yes → breaking change → must not ship as patch.

---

# 14. Practical Rule

If consumers can reasonably depend on a behavior, it is part of the contract.

When in doubt, preserve compatibility.
