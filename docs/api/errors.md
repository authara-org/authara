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
| `429` | Rate limit exceeded |
| `500` | Internal server error |

---

# Error Codes

The following error codes may be returned by Authara.

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `unauthorized` | 401 | The request does not contain a valid authenticated session |
| `invalid_request` | 400 | The request is malformed or missing required parameters |
| `csrf_invalid` | 403 | CSRF token is missing or invalid |
| `rate_limited` | 429 | Too many requests were made in a given time window |
| `internal_error` | 500 | An unexpected internal error occurred |

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

### `csrf_invalid`

Returned when a request requiring CSRF protection does not provide a valid token.

This typically occurs when:

- the `X-CSRF-Token` header is missing
- the token does not match the `authara_csrf` cookie

Example:

```json
{
  "error": {
    "code": "csrf_invalid",
    "message": "Invalid CSRF token"
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

See the full API contract:

https://github.com/authara-org/authara/blob/main/APICONTRACT.md
