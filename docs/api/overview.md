# API Overview

Authara exposes a versioned HTTP API used by applications and SDKs to interact
with authentication, accounts, sessions, and organizations.

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
[`contract/openapi.yaml`](https://github.com/authara-org/authara/blob/main/contract/openapi.yaml).

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

Browser clients normally rely on the **Authara session cookies**:

- `authara_access`
- `authara_refresh`

These cookies are automatically sent by the browser with each request.

Protected public endpoints accept either the `authara_access` cookie or an
`Authorization: Bearer <access-token>` header. If an Authorization header is
present but malformed or invalid, Authara does not fall back to the cookie.

Cookie-authenticated mutations require CSRF proof. Bearer-authenticated
mutations do not, because a bearer token is not ambient browser authority.

See **[Cookies](cookies.md)** for details about how Authara manages authentication cookies.

## Native token transport

Public authentication operations that declare the optional
`session_transport` query parameter support:

- `cookie` (default) — sets Authara session cookies and requires browser CSRF
  credentials for state-changing requests;
- `token` — returns the access and refresh tokens in JSON, sets no session
  cookies, and does not require browser CSRF credentials.

Token clients refresh through `POST /auth/api/v1/tokens/refresh`, revoke their
session through `POST /auth/api/v1/tokens/revoke`, and send the returned access
token as a bearer credential. Store the refresh token in platform secure
storage; do not use ordinary browser storage for it.

Native Google, Apple, and Microsoft options responses include a short-lived,
provider-bound `challenge_id`. Return it with the provider proof so Core can
validate the state and nonce without OAuth cookies. The browser cookie flow
remains the default.

---

# Response Format

Successful responses return standard JSON responses.

Example:

```json
{
  "id": "a8f7c1f5-5d2b-4a3a-91c5-1c87b6e19c41",
  "email": "user@example.com",
  "email_verified": true,
  "email_verified_at": "2026-01-01T11:30:00Z",
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

State-changing cookie-transport endpoints require a **CSRF token** for
protection against cross-site request forgery. This includes password auth,
signup challenges, and passkey ceremonies under `/auth/api/v1` when the client
uses cookies.

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
| OAuth and recovery | Authenticate with Google, Apple, or Microsoft and recover account collisions |
| Challenges | Verify signup, password-reset, and email-change codes |
| Passkeys | Authenticate with, register, and remove WebAuthn credentials |
| Account | Read and update the authenticated account, providers, sessions, and password |
| Organizations | Read or manage organizations, memberships, and invitations |
| Sessions | Refresh or revoke cookie and token sessions |

All endpoints are documented in **[Endpoints](endpoints.md)**.

---

# Intended Usage

The API is primarily used by:

- backend applications
- SDKs
- browser helpers
- native applications

Most applications will interact with Authara through:

- session cookies or bearer access tokens
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
