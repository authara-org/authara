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

Refreshes the current session and issues a new access token.

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
