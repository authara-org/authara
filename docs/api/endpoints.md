# API Endpoints

This document describes the JSON API endpoints exposed by Authara.

All endpoints are available under:

```
/auth/api/v1
```

These endpoints are primarily intended for:

- backend applications
- SDKs
- browser helpers

Authara also provides hosted HTML flows under `/auth`; applications that own
their authentication UI can use the JSON endpoints documented here.

Internal server-to-server endpoints are available under:

```
/auth/internal/v1
```

These endpoints are intended for your application backend, not browsers.

When `AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED=true`, authenticated
clients can use non-capacity organization-management endpoints under
`/auth/api/v1`. Organization creation, invitation creation, and invitation
resend remain internal server-to-server operations.

---

# Authentication

Public authentication endpoints do not require a session. Passkey registration
and user or organization endpoints require a valid session.

Authentication is performed using the `authara_access` cookie.

If the access token is missing or invalid, Authara returns:

```
401 Unauthorized
```

See [Cookies](cookies.md) for details.

All state-changing API routes require the `authara_csrf` cookie and a matching
`X-CSRF-Token` header, except `POST /auth/api/v1/tokens/refresh`. Obtain both by
calling `GET /auth/api/v1/csrf` first.

---

# Endpoints

## Get a CSRF token

```text
GET /auth/api/v1/csrf
```

Returns `200 OK`, sets the `authara_csrf` cookie, and returns its value:

```json
{
  "csrf_token": "<csrf-token>"
}
```

Send this value as `X-CSRF-Token` on the POST requests below while preserving
the cookie.

---

## Log in with Google

Google login must be enabled with `AUTHARA_OAUTH_PROVIDERS=google`.

First initialize the login attempt:

```text
GET /auth/api/v1/oauth/google/options
```

The response sets the HttpOnly `authara_oauth_nonce` cookie and returns:

```json
{
  "client_id": "<google-client-id>",
  "nonce": "<nonce>"
}
```

Pass both values to Google Identity Services. After Google returns its ID-token
credential, exchange it while preserving the nonce cookie:

```text
POST /auth/api/v1/oauth/google?audience=app
X-CSRF-Token: <csrf-token>
Content-Type: application/json
```

```json
{
  "credential": "<google-id-token>",
  "nonce": "<nonce>"
}
```

`audience` is optional and defaults to `app`; supported values are `app` and
`admin`. On success Authara returns the same user, access-token, and
refresh-token response as password login and sets both session cookies.

React clients should call both the CSRF and Google-options endpoints with
credentials enabled. Mobile clients must likewise preserve and return the
cookies set by these endpoints; the returned access and refresh tokens can
then be stored by the client.

If the Google email belongs to an existing account that has not linked Google,
the endpoint returns `409 account_link_required`. The user must sign in using
an existing method and link Google from the account page; Authara never links
accounts based only on a matching email address.

Errors: `400 invalid_request`, `401 unauthorized`, `403 forbidden`,
`404 not_found`, `409 account_link_required`, or `500 internal_error`.

---

## Log in with a password

```text
POST /auth/api/v1/login?audience=app
```

`audience` is optional and defaults to `app`; supported values are `app` and
`admin`.

```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

On success Authara returns `200 OK`, sets `authara_access` and
`authara_refresh`, and returns:

```json
{
  "user": {
    "id": "a8f7c1f5-5d2b-4a3a-91c5-1c87b6e19c41",
    "email": "user@example.com",
    "username": "user",
    "disabled": false,
    "created_at": "2026-01-01T12:00:00Z"
  },
  "access_token": "<access-token>",
  "refresh_token": "<refresh-token>"
}
```

Errors: `400 invalid_request`, `401 unauthorized`, `403 forbidden`,
`429 rate_limited`, or `500 internal_error`.

---

## Sign up directly

```text
POST /auth/api/v1/signup/direct
```

The request body is the same email-and-password object used for login. Signup
only creates app-audience sessions. Authara creates the account, returns
`201 Created` with the authentication response shown above, and sets both
session cookies.

This operation returns `404 not_found` when challenges are enabled.

Errors: `400 invalid_request`, `403 forbidden`, `404 not_found`, `429
rate_limited`, or `500 internal_error`.

---

## Start signup verification

```text
POST /auth/api/v1/signup/challenges
```

The request body is the same email-and-password object used for direct signup.
Authara starts email verification and returns `202 Accepted` without creating a
session:

```json
{
  "challenge_id": "49f7a8b7-5f13-4ab0-9991-e924566a08ba"
}
```

This operation returns `404 not_found` when challenges are disabled.

Errors: `400 invalid_request`, `403 forbidden`, `404 not_found`, `429
rate_limited`, or `500 internal_error`.

---

## Verify signup verification

```text
POST /auth/api/v1/signup/challenges/verify
```

```json
{
  "challenge_id": "49f7a8b7-5f13-4ab0-9991-e924566a08ba",
  "code": "123456"
}
```

On success Authara creates the account, returns `201 Created` with the
authentication response, and sets both session cookies.

Errors: `400 invalid_request`, `403 forbidden`, `404 not_found` when challenges
are disabled, `429 rate_limited`, or `500 internal_error`.

---

## Start password reset

```text
POST /auth/api/v1/password-reset/challenges
```

```json
{
  "email": "user@example.com",
  "new_password": "new-password123"
}
```

Returns `202 Accepted` with a `challenge_id`. The response is identical when
the email does not belong to an account, preventing account enumeration. This
endpoint remains available when optional challenge-based signup is disabled.

Errors: `400 invalid_request`, `403 forbidden`, `429 rate_limited`, or
`500 internal_error`.

---

## Verify password reset

```text
POST /auth/api/v1/password-reset/challenges/verify
```

```json
{
  "challenge_id": "49f7a8b7-5f13-4ab0-9991-e924566a08ba",
  "code": "123456"
}
```

Returns `204 No Content`. The password is changed and all existing sessions
for the account are revoked. The user must log in with the new password. This
endpoint remains available when optional challenge-based signup is disabled.

Errors: `400 invalid_request`, `403 forbidden`, `429 rate_limited`, or
`500 internal_error`.

---

## Resend a verification code

```text
POST /auth/api/v1/challenges/resend
```

```json
{
  "challenge_id": "49f7a8b7-5f13-4ab0-9991-e924566a08ba"
}
```

Returns `204 No Content`. Unknown, expired, consumed, too-recent, and
resend-exhausted challenges also return `204`; this intentionally
prevents clients from using resend responses to discover account or challenge
state.

Malformed requests return `400 invalid_request`; failed CSRF validation returns
`403 forbidden`; IP rate limiting returns `429 rate_limited`; unexpected
failures return `500 internal_error`.

---

## Authenticate with a passkey

Start a WebAuthn authentication ceremony:

```text
POST /auth/api/v1/passkeys/authenticate/options
```

The request body is empty. Authara returns `200 OK` with WebAuthn options:

```json
{
  "challenge_id": "7e9cbd7c-a532-4b35-bf1b-3c5237bfb760",
  "options": {
    "publicKey": {}
  }
}
```

Before calling `navigator.credentials.get()`, convert the base64url-encoded
`challenge` and credential IDs in `options.publicKey` to `BufferSource` values.
After the browser ceremony, serialize the credential's binary fields back to
base64url strings and submit it:

```text
POST /auth/api/v1/passkeys/authenticate/finish?audience=app
```

```json
{
  "challenge_id": "7e9cbd7c-a532-4b35-bf1b-3c5237bfb760",
  "credential": {}
}
```

`audience` is optional and defaults to `app`. On success Authara returns
`200 OK` with the authentication response and sets both session cookies.

Both endpoints require a valid API CSRF token and may return `403 forbidden`.
The options endpoint may also return `429 rate_limited` or `500 internal_error`.
The finish endpoint may return `400 invalid_request`, `401 unauthorized`,
`429 rate_limited`, or `500 internal_error`.

---

## Register a passkey

Both registration endpoints require an authenticated app session.

```text
POST /auth/api/v1/passkeys/register/options
```

The request body is empty. Authara returns the same `challenge_id` and `options`
envelope shown above. Decode its base64url binary fields before passing
`options.publicKey` to `navigator.credentials.create()`, then encode the
credential's binary response fields as base64url before submitting it:

```text
POST /auth/api/v1/passkeys/register/finish
```

```json
{
  "challenge_id": "7e9cbd7c-a532-4b35-bf1b-3c5237bfb760",
  "credential": {},
  "name": "Work laptop",
  "platform_hint": "macOS"
}
```

`name` and `platform_hint` are optional. Success returns `204 No Content`.

Both endpoints require the access and CSRF cookies. The options endpoint may
return `401 unauthorized`, `403 forbidden`, or `500 internal_error`.
The finish endpoint may return `400 invalid_request`, `401 unauthorized`,
`403 forbidden`, `409 passkey_already_exists`,
`422 passkey_registration_invalid`, or `500 internal_error`.

---

## Get current user

Returns information about the authenticated user.

```
GET /auth/api/v1/user
```

### Authentication

Required.

### Response

```
200 OK
```

Example:

```json
{
  "id": "a8f7c1f5-5d2b-4a3a-91c5-1c87b6e19c41",
  "email": "user@example.com",
  "username": "user",
  "roles": [],
  "organization": {
    "id": "68c673e7-1ff9-4113-8bbf-e00f039a9a61",
    "name": "user",
    "role": "owner"
  },
  "disabled": false,
  "created_at": "2026-01-01T12:00:00Z"
}
```

### Errors

| Status | Code |
|------|------|
| 401 | unauthorized |

See [Errors](errors.md) for error definitions.

---

## Set current user password

Creates or replaces the authenticated user's password. The user ID is taken
from the access token; clients cannot supply one.

```text
PUT /auth/api/v1/users/password
Content-Type: application/json
```

```json
{
  "password": "new-password123"
}
```

Requires the access and CSRF cookies. Returns `204 No Content` and revokes all
existing sessions and refresh tokens. Errors: `400 invalid_request`,
`401 unauthorized`, `403 forbidden`, or `500 internal_error`.

---

## Refresh session

Refreshes the current browser cookie session and issues a new access cookie.

```
POST /auth/api/v1/sessions/refresh
```

### Authentication

Requires the `authara_refresh` cookie.

### Request

The request must include a CSRF token.

Example header:

```
X-CSRF-Token: <csrf-token>
```

The token must match the value stored in the `authara_csrf` cookie.

See [Cookies](cookies.md) for details.

### Query Parameters

| Parameter | Required | Description |
|------|------|------|
| `audience` | yes | Requested token audience |

Example:

```
POST /auth/api/v1/sessions/refresh?audience=app
```

### Response

```
200 OK
```

The response body is empty.

New session cookies are issued:

- `authara_access`
- `authara_refresh` (depending on rotation policy)

### Errors

| Status | Code |
|------|------|
| 401 | unauthorized |
| 400 | invalid_request |
| 500 | internal_error |

See [Errors](errors.md).

---

## Refresh tokens

Refreshes a token session without cookies.

```
POST /auth/api/v1/tokens/refresh
```

### Request

```json
{
  "refresh_token": "<refresh-token>",
  "audience": "app"
}
```

### Response

```json
{
  "access_token": "<access-token>",
  "refresh_token": "<refresh-token>"
}
```

### Errors

| Status | Code |
|------|------|
| 401 | unauthorized |
| 400 | invalid_request |
| 500 | internal_error |

See [Errors](errors.md).

---

# Organization Management

Set `AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED=true` to allow authenticated
clients to manage non-capacity organization data directly through Authara.
Organization creation, invitation creation, invitation resend, membership
removal, organization deletion, and user deletion remain internal-only so your
application backend can enforce billing, subscriptions, seat limits, and
product-data dependencies.

Public organization routes require the `authara_access` cookie. State-changing
requests also require the API CSRF token. Organization-scoped routes only accept
the organization from the current access token. Authara ignores client-supplied
actor IDs and applies current membership and owner/admin checks.

Available routes:

```text
GET   /auth/api/v1/capabilities
GET   /auth/api/v1/organizations/{organizationID}
PATCH /auth/api/v1/organizations/{organizationID}
GET   /auth/api/v1/organizations/{organizationID}/members
GET   /auth/api/v1/organizations/{organizationID}/members/{userID}
GET   /auth/api/v1/organizations/{organizationID}/invitations
GET   /auth/api/v1/organizations/{organizationID}/invitations/{invitationID}
POST  /auth/api/v1/organizations/{organizationID}/invitations/{invitationID}/revoke
GET   /auth/api/v1/users/{userID}/memberships
```

The capabilities route remains available to authenticated clients when direct
management is disabled and reports
`allows_public_organization_management: false`. The other routes return `404`
while the feature is disabled.

In `single` mode, `allows_organization_leave` is `true` because a departure can
be approved by the application backend, while `allows_org_switching` remains
`false`: the user may never hold two memberships and cannot switch between
simultaneously available organizations.

---

# Internal Endpoints

The internal API contains server-to-server lifecycle operations whose
product-specific checks belong to the application backend. A webhook cannot
veto an operation after it happened, so the application must validate billing
and dependencies before calling these endpoints.

## Create organization

Creates a team organization on behalf of a user after your application backend
has enforced product-specific rules.

```text
POST /auth/internal/v1/organizations
Authorization: Bearer <AUTHARA_INTERNAL_API_TOKEN>
```

### Request

```json
{
  "name": "Example Inc",
  "created_by_user_id": "8d0b28cc-f307-4f0b-8f61-c5c9f736c4b1"
}
```

## Remove an organization member

Removes a membership after the application backend has approved the departure.
Set `actor_user_id` to the target user for a voluntary leave, or to the owner or
admin performing a removal.

```text
DELETE /auth/internal/v1/organizations/{organization_id}/members/{user_id}
Authorization: Bearer <AUTHARA_INTERNAL_API_TOKEN>
```

```json
{
  "actor_user_id": "8d0b28cc-f307-4f0b-8f61-c5c9f736c4b1"
}
```

In `multi` mode, non-creators may leave or be removed from a personal
organization; its creator cannot leave. In `personal` mode, invitations are
disabled, so no additional member can join. Authara also rejects removal of the
last member or sole owner. Sessions currently using the removed organization
and their refresh tokens are deleted; access tokens for that user and
organization are revoked. The operation emits
`organization.membership.deleted`.

## Transfer organization ownership

Atomically promotes an existing member to owner and demotes the current owner
to admin. The actor must be an owner, the new owner must already belong to the
team organization, and both users are protected from concurrent deletion while
the transfer runs.

```text
POST /auth/internal/v1/organizations/{organization_id}/ownership-transfer
Authorization: Bearer <AUTHARA_INTERNAL_API_TOKEN>
```

```json
{
  "actor_user_id": "8d0b28cc-f307-4f0b-8f61-c5c9f736c4b1",
  "new_owner_user_id": "67a1123a-e04f-44c2-aae9-314e28dcbd9c"
}
```

Personal organization ownership cannot be transferred. A successful transfer
emits `organization.membership.updated` for both memberships and allows the
former owner to leave through the member-removal endpoint.

## Delete an organization

Permanently deletes a team organization after application-owned subscription
and data checks have passed. Only an owner may perform this operation.

```text
DELETE /auth/internal/v1/organizations/{organization_id}
Authorization: Bearer <AUTHARA_INTERNAL_API_TOKEN>
```

```json
{
  "actor_user_id": "8d0b28cc-f307-4f0b-8f61-c5c9f736c4b1"
}
```

Personal organizations cannot be deleted through this endpoint. In `single`
mode, organization deletion is accepted only when the actor is the sole member;
an organization with other members requires a separately designed tenant
teardown. In `multi` mode, all memberships may be removed because users retain
their personal organizations. A successful deletion removes organization-bound
sessions and invitations and emits `organization.deleted`.

## Delete a user

Permanently deletes a user after account-level application checks have passed.

```text
DELETE /auth/internal/v1/users/{user_id}
Authorization: Bearer <AUTHARA_INTERNAL_API_TOKEN>
```

User deletion cannot bypass organization invariants: Authara rejects deletion
when the user is the last member or sole owner of a team organization. An owned
personal organization is deleted as part of the user deletion; in `multi` mode,
memberships in other users' personal organizations are removed. Direct deletion
from Authara's hosted account page is disabled by default; enable it explicitly
with `AUTHARA_PUBLIC_ACCOUNT_DELETION_ENABLED=true` only when application-level
checks are unnecessary.

## Create organization invitation

Creates a secure Authara organization invitation and returns the invitation link.

Your application backend should call this after it has enforced product-specific rules such as billing and seat limits. `actor_user_id` is required; Authara validates that the actor is a member of the organization and is allowed to invite members. `role` controls the Authara organization role granted on acceptance and may be `member` or `admin`; it defaults to `member`. The `owner` role remains reserved for the organization creator. `metadata` is optional opaque application JSON that Authara stores and forwards unchanged.

```
POST /auth/internal/v1/organizations/{organization_id}/invitations
Authorization: Bearer <AUTHARA_INTERNAL_API_TOKEN>
```

### Request

```json
{
  "actor_user_id": "8d0b28cc-f307-4f0b-8f61-c5c9f736c4b1",
  "email": "teammate@example.com",
  "role": "admin",
  "metadata": {
    "baufunk": {
      "role": "manager"
    }
  }
}
```

### Response

```json
{
  "invitation": {
    "id": "7ea9ce22-72bb-45bd-96d2-7368314dd345",
    "organization_id": "68c673e7-1ff9-4113-8bbf-e00f039a9a61",
    "email": "teammate@example.com",
    "role": "admin",
    "metadata": {
      "baufunk": {
        "role": "manager"
      }
    },
    "status": "pending",
    "expires_at": "2026-01-08T12:00:00Z",
    "invite_url": "https://example.com/auth/invitations/accept?token=..."
  }
}
```

Authara also enqueues an invitation email when the email worker is configured. The returned `invite_url` is always present for testing or app-owned delivery.

In `single` mode, an existing account may accept an invitation only after its
previous membership has been removed. The hosted invitation page keeps the
invitation pending and tells the user to leave or delete the current organization
through the application. After the application backend approves that operation
and calls the internal API, the user can reopen the invitation, log in, and
accept it normally.

## Resend organization invitation

Replaces an existing invitation with a fresh pending invitation.

```text
POST /auth/internal/v1/organizations/{organization_id}/invitations/{invitation_id}/resend
Authorization: Bearer <AUTHARA_INTERNAL_API_TOKEN>
```

### Errors

| Status | Code |
|------|------|
| 401 | unauthorized |
| 403 | actor_not_member |
| 403 | actor_not_allowed |
| 404 | organization_not_found |
| 409 | already_member |
| 409 | invitation_already_pending |
| 400 | invalid_request |
| 500 | internal_error |

---

# Versioning

Authara uses path-based versioning.

Current version:

```
/auth/api/v1
```

Future versions may introduce new endpoints under `/auth/api/v2`.

---

# Summary

Authara exposes a minimal API focused on:

- retrieving the authenticated user
- managing the authenticated user's organization when enabled
- refreshing sessions

Additional endpoints may be introduced in future versions.
