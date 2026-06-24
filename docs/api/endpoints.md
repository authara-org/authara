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

Browser login flows use HTML endpoints under `/auth`.

Internal server-to-server endpoints are available under:

```
/auth/internal/v1
```

These endpoints are intended for your application backend, not browsers.

---

# Authentication

Most endpoints require a valid session.

Authentication is performed using the `authara_access` cookie.

If the access token is missing or invalid, Authara returns:

```
401 Unauthorized
```

See [Cookies](cookies.md) for details.

---

# Endpoints

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

# Internal Endpoints

## Create organization invitation

Creates a secure Authara member invitation object and returns the invitation link.

Your application backend should call this after it has enforced product-specific rules such as billing and seat limits. `actor_user_id` is required; Authara validates that the actor is a member of the organization and is allowed to invite members. Invitations always grant the `member` role; upgrade roles separately after the user joins.

```
POST /auth/internal/v1/organizations/{organization_id}/invitations
Authorization: Bearer <AUTHARA_INTERNAL_API_TOKEN>
```

### Request

```json
{
  "actor_user_id": "8d0b28cc-f307-4f0b-8f61-c5c9f736c4b1",
  "email": "teammate@example.com",
  "return_to": "/settings/team"
}
```

### Response

```json
{
  "invitation": {
    "id": "7ea9ce22-72bb-45bd-96d2-7368314dd345",
    "organization_id": "68c673e7-1ff9-4113-8bbf-e00f039a9a61",
    "email": "teammate@example.com",
    "role": "member",
    "status": "pending",
    "expires_at": "2026-01-08T12:00:00Z",
    "invite_url": "https://example.com/auth/invitations/accept?token=..."
  }
}
```

Authara also enqueues an invitation email when the email worker is configured. The returned `invite_url` is always present for testing or app-owned delivery.

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
- refreshing sessions

Additional endpoints may be introduced in future versions.
