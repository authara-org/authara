# API Errors

Authara returns errors using a consistent JSON envelope.

All API endpoints use the same structure for error responses.

---

# Error Response Format

Errors are returned as a JSON object containing an `error` field.

Example:

```json
{
  "error": {
    "code": "unauthorized",
    "message": "Invalid refresh token"
  }
}
```

Fields:

| Field | Description |
|------|-------------|
| `error.code` | Machine-readable error code |
| `error.message` | Human-readable error description |

Applications should rely primarily on the **error code**, not the message.

---

# HTTP Status Codes

Authara uses standard HTTP status codes together with the error envelope.

Common status codes include:

| Status | Meaning |
|------|------|
| `400` | Bad request |
| `401` | Authentication required or invalid session |
| `403` | Access forbidden |
| `404` | Resource not found |
| `409` | Request conflicts with existing state |
| `422` | Well-formed input could not be verified |
| `429` | Rate limit exceeded |
| `500` | Internal server error |

---

# Error Codes

The following error codes may be returned by Authara.

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `unauthorized` | 401 | The request does not contain a valid authenticated session |
| `invalid_request` | 400 | The request is malformed or missing required parameters |
| `account_link_required` | 409 | The external identity matches an existing account and must be linked explicitly |
| `forbidden` | 403 | The request is not allowed, including CSRF validation failures |
| `not_found` | 404 | The requested resource or enabled feature is not available |
| `passkey_already_exists` | 409 | The passkey is already linked to an account |
| `passkey_registration_invalid` | 422 | The passkey registration ceremony could not be verified |
| `rate_limited` | 429 | Too many requests were made in a given time window |
| `internal_error` | 500 | An unexpected internal error occurred |
| `actor_not_member` | 403 | The internal lifecycle actor is not a member of the organization |
| `actor_not_allowed` | 403 | The actor's organization role cannot perform the operation |
| `last_organization_owner` | 409 | Ownership must be transferred before the member can leave, be removed, or be deleted |
| `last_organization_member` | 409 | The last member must explicitly delete the organization |
| `organization_has_other_members` | 409 | A single-mode organization with other members cannot use the ordinary delete operation |
| `personal_organization_immutable` | 409 | A personal-organization creator cannot leave, and the organization cannot be deleted independently of its user |
| `last_active_admin` | 409 | The final active platform administrator cannot be deleted |

---

# Authentication Errors

These errors are related to session validation.

### `unauthorized`

Returned when:

- the `authara_access` cookie is missing
- the access token is invalid
- the session has expired
- the refresh token is invalid

Example:

```json
{
  "error": {
    "code": "unauthorized",
    "message": "Authentication required"
  }
}
```

---

# CSRF Errors

### `forbidden`

Returned when a request requiring CSRF protection does not provide a valid token.

This typically occurs when:

- the `X-CSRF-Token` header is missing
- the token does not match the `authara_csrf` cookie

Example:

```json
{
  "error": {
    "code": "forbidden",
    "message": "CSRF validation failed"
  }
}
```

See:

- [Cookies](cookies.md)

---

# Rate Limiting

Authara may reject requests when rate limits are exceeded.

### `rate_limited`

Returned with:

```
429 Too Many Requests
```

Example:

```json
{
  "error": {
    "code": "rate_limited",
    "message": "Too many login attempts"
  }
}
```

---

# Internal Errors

### `internal_error`

Returned when an unexpected server error occurs.

Example:

```json
{
  "error": {
    "code": "internal_error",
    "message": "Internal server error"
  }
}
```

Applications should treat this as a temporary failure.

---

# Stability

The **error envelope format** and **error codes** are part of the Authara API contract.

Applications may rely on these codes remaining stable within a given API version.

See the full [`contract/openapi.yaml`](../../contract/openapi.yaml) API contract.
