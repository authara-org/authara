# API Overview

Authara exposes a small HTTP API used by applications and SDKs to interact with the authentication system.

The API is designed to be:

- simple
- predictable
- stable across versions

Most browser-based authentication flows use **HTML endpoints** under `/auth`, while programmatic integrations use **JSON endpoints** under `/auth/api/v1`.

---

# API Contract

Authara maintains a formal **API contract** that defines the stability guarantees of the public HTTP interface.

This contract covers:

- endpoint paths
- response structures
- cookie names
- error envelope format
- versioning rules

The contract is defined in
[`contract/http.yaml`](../../contract/http.yaml).

Applications and SDKs may rely on this document when integrating with Authara.

---

# Base Path

Authara is typically mounted under:

```
/auth
```

Example routing:

```
/auth/* → Authara
/*      → Application
```

All API endpoints are therefore available under:

```
/auth/api/v1
```

Example:

```
/auth/api/v1/user
```

A full list of available endpoints can be found in the **[Endpoints](endpoints.md)** documentation.

---

# Authentication Model

Authenticated API endpoints rely on the **Authara session cookies**:

- `authara_access`
- `authara_refresh`

These cookies are automatically sent by the browser with each request.

Password, passkey, and Google login endpoints also return the access and refresh
tokens in JSON. Authenticated v1 endpoints currently read the access token from
the `authara_access` cookie; they do not accept an `Authorization: Bearer`
header. Non-browser clients therefore need a cookie-aware HTTP client.

Authentication is performed using the **access token** contained in the `authara_access` cookie.

See **[Cookies](cookies.md)** for details about how Authara manages authentication cookies.

---

# Response Format

Successful responses return standard JSON responses.

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

The exact structure depends on the endpoint.

See **[Endpoints](endpoints.md)** for endpoint-specific response formats.

---

# Error Responses

Errors are returned using a consistent JSON envelope.

Example:

```json
{
  "error": {
    "code": "unauthorized",
    "message": "Invalid refresh token"
  }
}
```

The structure and available error codes are documented in **[Errors](errors.md)**.

---

# CSRF Protection

State-changing browser-session endpoints require a **CSRF token** for
protection against cross-site request forgery. This includes password auth,
signup challenges, and passkey ceremonies under `/auth/api/v1`.

For browser requests, the token must be provided either:

- as the `X-CSRF-Token` request header
- as a hidden form field

The value must match the `authara_csrf` cookie.

See **[Cookies](cookies.md)** for details about the CSRF cookie and **[Errors](errors.md)** for CSRF-related error responses.

---

# API Versioning

Authara uses **path-based versioning**.

Current version:

```
/auth/api/v1
```

This allows future API versions to evolve without breaking existing integrations.

---

# Endpoint Categories

The API currently includes endpoints for:

| Category | Purpose |
|------|------|
| Authentication | Log in and sign up with passwords or passkeys |
| Challenges | Verify signup email codes and request another code |
| Passkeys | Authenticate with or register WebAuthn credentials |
| User | Retrieve the authenticated user |
| Session | Refresh the current session |

All endpoints are documented in **[Endpoints](endpoints.md)**.

---

# Intended Usage

The API is primarily used by:

- backend applications
- SDKs
- browser helpers

Most applications will interact with Authara through:

- session cookies
- hosted redirect-based flows or the versioned JSON authentication API
- the `/auth/api/v1/user` endpoint

---

# Summary

Authara provides a stable HTTP API for authentication, signup verification,
passkeys, user state, organizations, and sessions.

For detailed information see:

- **[Endpoints](endpoints.md)**
- **[Cookies](cookies.md)**
- **[Errors](errors.md)**
