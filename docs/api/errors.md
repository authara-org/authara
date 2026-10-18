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

Some errors add typed recovery data alongside `error`:

- `recent_authentication_required` includes `authentication_challenge` and
  `reauthenticate_url`;
- `account_link_required` includes `recovery` with the pending provider,
  short-lived link ID, and available proof methods.

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
| `428` | A fresh authentication proof is required before the mutation can proceed |
| `429` | Rate limit exceeded |
| `500` | Internal server error |
| `503` | A fail-closed authentication dependency is temporarily unavailable |

---

# Error Codes

The following error codes may be returned by Authara.

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `account_link_required` | 409 | The external identity matches an existing account and must be linked explicitly |
| `actor_not_allowed` | 403 | The internal lifecycle actor's organization role cannot perform the operation |
| `actor_not_member` | 403 | The internal lifecycle actor is not a member of the organization |
| `already_member` | 409 | The invitation target already belongs to the organization |
| `auth_method_already_linked` | 409 | The authentication method is already linked to an account |
| `cannot_remove_last_auth_method` | 409 | Removing the method would leave the account without a sign-in method |
| `forbidden` | 403 | The request is not allowed, including CSRF validation failures |
| `internal_error` | 500 or 503 | The operation failed internally or a fail-closed dependency is unavailable |
| `invalid_authentication_challenge` | 409 | The authentication challenge is expired, consumed, or does not belong to the current session |
| `invalid_request` | 400 | The request is malformed or missing required parameters |
| `invitation_already_accepted` | 409 | The invitation has already been accepted |
| `invitation_already_pending` | 409 | A pending invitation already exists for the email and organization |
| `invitation_email_mismatch` | 403 | The authenticated account does not match the invited email address |
| `invitation_expired` | 409 | The invitation has expired |
| `invitation_flow_mismatch` | 409 | The request does not match the invitation's required login or signup flow |
| `invitation_not_found` | 404 | The invitation does not exist or is unavailable |
| `invitation_revoked` | 409 | The invitation has been revoked |
| `last_active_admin` | 409 | The final active platform administrator cannot be disabled, demoted, or deleted |
| `last_organization_member` | 409 | The last member must explicitly delete the organization |
| `last_organization_owner` | 409 | Ownership must be transferred before the member can leave, be removed, or be deleted |
| `membership_not_found` | 404 | The organization membership does not exist |
| `not_found` | 404 | The requested resource or enabled feature is not available |
| `organization_has_other_members` | 409 | A single-mode organization with other members cannot use the ordinary delete operation |
| `organization_membership_conflict` | 409 | The requested membership conflicts with the configured organization mode |
| `organization_not_found` | 404 | The organization does not exist or is unavailable |
| `passkey_already_exists` | 409 | The passkey is already linked to an account |
| `passkey_registration_invalid` | 422 | The passkey registration ceremony could not be verified |
| `password_already_exists` | 409 | The account already has a password method |
| `password_provider_not_found` | 404 | The account does not have a password method to change |
| `personal_organization_immutable` | 409 | A personal-organization creator cannot leave, and the organization cannot be deleted independently of its user |
| `provider_link_expired` | 409 | The pending external-provider recovery link expired or was consumed |
| `rate_limited` | 429 | Too many requests were made in a given time window |
| `recent_authentication_required` | 428 | The session is valid but its last password, passkey, or federated proof is too old for a sensitive mutation |
| `unauthorized` | 401 | The request does not contain a valid authenticated session or credential |
| `user_not_found` | 404 | The user does not exist or is unavailable |
| `username_taken` | 409 | The requested username already belongs to another user |

---

# Authentication Errors

These errors are related to session validation.

### `unauthorized`

Returned when:

- the `authara_access` cookie or bearer credential is missing
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

Returned when a cookie-authenticated request requiring CSRF protection does
not provide a valid token.

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

Returned with `500` when an unexpected server error occurs. Authara can also
return this code with `503` when a security dependency such as the immediate
revocation store is unavailable and the request must fail closed.

Example:

```json
{
  "error": {
    "code": "internal_error",
    "message": "Internal server error"
  }
}
```

Applications should treat this as a temporary failure. Retry only when the
operation is safe to repeat.

---

# Stability

The **error envelope format** and **error codes** are part of the Authara API contract.

Applications may rely on these codes remaining stable within a given API version.

See the full
[`contract/openapi.yaml`](https://github.com/authara-org/authara/blob/main/contract/openapi.yaml)
API contract for the codes declared by each operation.
