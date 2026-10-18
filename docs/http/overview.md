# HTTP Interface

Authara exposes an HTTP interface that is intended for both **browser-based authentication flows** and **programmatic access by applications**.

The HTTP surface is divided into two categories:

- **Browser routes** – HTML pages and browser authentication flows
- **JSON API** – programmatic endpoints used by applications and SDKs

---

# Base Path

Authara is typically mounted under the `/auth` path behind a reverse proxy or gateway.

Example routing:

```
/auth/*  → Authara
/*       → Application
```

This allows Authara to run alongside an existing application without taking over the root domain.

---

# Browser Routes

Browser routes provide the built-in authentication interface used by end users.

Examples include:

```
GET  /auth/login
GET  /auth/signup
GET  /auth/account
POST /auth/sessions/logout
```

These endpoints return **HTML pages** and are intended to be accessed directly by a web browser.

Typical usage pattern:

1. A user requests a protected page in the application
2. The application detects the user is not authenticated
3. The application redirects the user to `/auth/login`
4. Authara performs the login flow
5. The user is redirected back to the application

Browser routes may accept query parameters such as:

```
/auth/login?return_to=/dashboard
```

The `return_to` parameter specifies where the user should be redirected after successful authentication.

For detailed behavior of browser routes, see:

- [Browser Routes](browser-routes.md)

---

# JSON API

Authara also exposes a versioned JSON API for programmatic access.

All API endpoints are available under:

```
/auth/api/v1
```

These endpoints are primarily used by:

- backend applications
- SDKs
- browser helper libraries
- integration services

Examples:

```
GET  /auth/api/v1/user
POST /auth/api/v1/sessions/refresh
POST /auth/api/v1/tokens/refresh
```

Browser clients normally use cookie sessions. Native and other non-browser
clients can request `session_transport=token` on authentication operations,
refresh through `/auth/api/v1/tokens/refresh`, and authenticate protected API
calls with `Authorization: Bearer <access-token>`.

See the API documentation for details:

- [API Overview](../api/overview.md)
- [API Endpoints](../api/endpoints.md)

---

# Authentication

Browser API requests normally rely on **session cookies** issued by Authara.

The most important cookies include:

- `authara_access` – short-lived access token
- `authara_refresh` – refresh token
- `authara_csrf` – CSRF protection token

For full details, see:

- [Cookies](../api/cookies.md)

Token-transport clients keep the returned access and refresh tokens in their
platform's secure storage instead. Bearer-authenticated mutations do not
require browser CSRF proof; cookie-authenticated mutations still do.

---

# CSRF Protection

State-changing requests require CSRF protection.

Cookie-authenticated clients must include the CSRF token in the request header:

```
X-CSRF-Token: <csrf-token>
```

The token must match the value stored in the `authara_csrf` cookie.

The token may also be submitted through a hidden form field in HTML forms.

See:

- [Cookies](../api/cookies.md)

---

# Security Headers

Authara sets security headers on its HTTP responses by default:

- `Content-Security-Policy`
- `X-Frame-Options`
- `X-Content-Type-Options`
- `Referrer-Policy`

These headers harden the built-in authentication UI and API responses against common browser-side risks such as clickjacking, MIME sniffing, referrer leakage, and unexpected script or asset loading.

When Google, Apple, or Microsoft OAuth is enabled, the Content Security Policy
allows only the corresponding official browser SDK origins required by login,
account-linking, and reauthentication UI.

---

# Versioning

The JSON API uses **path-based versioning**.

Current version:

```
/auth/api/v1
```

Future versions may introduce additional endpoints under new paths such as:

```
/auth/api/v2
```

Existing versions will remain stable for compatibility.

---

# Summary

Authara's HTTP interface consists of two clearly separated layers:

- **Browser routes** for authentication flows
- **JSON API** for programmatic access

Applications typically interact with Authara by:

- redirecting users to browser routes such as `/auth/login`
- consuming authenticated session state through cookies or bearer tokens
- optionally calling the JSON API when needed
