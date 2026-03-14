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

The contract is defined in the repository:

**APICONTRACT.md**

https://github.com/authara-org/authara/blob/main/APICONTRACT.md

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

API endpoints rely on the **Authara session cookies**:

- `authara_access`
- `authara_refresh`

These cookies are automatically sent by the browser with each request.

Applications do not need to manually attach tokens.

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

Some endpoints require a **CSRF token** for protection against cross-site request forgery.

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
- redirect-based login flows
- the `/auth/api/v1/user` endpoint

---

# Summary

Authara provides a small, stable HTTP API for retrieving authentication state and managing sessions.

For detailed information see:

- **[Endpoints](endpoints.md)**
- **[Cookies](cookies.md)**
- **[Errors](errors.md)**
