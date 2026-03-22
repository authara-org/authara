# Cookies

Authara uses HTTP cookies to manage browser sessions and protect state-changing requests.

These cookies are issued by Authara and automatically sent by the browser with each request to the Authara endpoints.

---

# Overview

Authara uses three cookies:

| Cookie | Purpose |
|------|------|
| `authara_access` | Short-lived access token used for request authentication |
| `authara_refresh` | Refresh token used to obtain new access tokens |
| `authara_csrf` | CSRF protection token for browser POST requests |

All authentication state is stored in cookies.  
Applications do not need to store tokens manually.

---

# authara_access

The `authara_access` cookie contains the **JWT access token**.

Properties:

- **short-lived**
- **HTTP-only**
- **sent with every request**
- used by APIs and middleware to authenticate requests

Example characteristics:

| Property | Value |
|------|------|
| HTTPOnly | yes |
| Secure | yes (in production) |
| SameSite | Lax |
| Path | `/` |

The access token typically expires after **10 minutes**.

---

# authara_refresh

The `authara_refresh` cookie contains the **refresh token**.

Refresh tokens allow the client to obtain new access tokens without requiring the user to log in again.

Properties:

| Property | Value |
|------|------|
| HTTPOnly | yes |
| Secure | yes (in production) |
| SameSite | Lax |
| Path | `/` |

Important characteristics:

- opaque random token
- stored **hashed** in the database
- rotated periodically or on every use depending on configuration
- reuse detection triggers session invalidation

Refresh tokens are **never accessible from JavaScript**.

---

# authara_csrf

The `authara_csrf` cookie contains the CSRF protection token.

Unlike the other cookies, this cookie **is readable by JavaScript** so that applications can include the token in protected requests.

Properties:

| Property | Value |
|------|------|
| HTTPOnly | no |
| Secure | yes (in production) |
| SameSite | Lax |
| Path | `/` |

---

# CSRF Protection

State-changing requests (for example login, logout, or refresh) require the CSRF token to be provided with the request.

The token must match the value stored in the `authara_csrf` cookie.

There are two supported ways to include the token.

---

## Header

For API-style requests (for example `fetch` or `XMLHttpRequest`), the token should be sent in the `X-CSRF-Token` header.

Example:

```
X-CSRF-Token: <csrf-token>
```

Browser integrations typically read the CSRF cookie and attach the header automatically.

The `@authara/browser` SDK provides helpers for this.

---

## Hidden form field

For traditional HTML form submissions and SSR apps, the CSRF token may be included as a hidden form field.

Example:

```html
<form method="POST" action="/auth/logout">
  <input type="hidden" name="csrf_token" value="<csrf-token>">
  <button type="submit">Logout</button>
</form>
```

The token value must match the value stored in the `authara_csrf` cookie.
---

# Cookie Security

Authara configures cookies to follow common security best practices:

- authentication cookies are **HTTP-only**
- cookies use **SameSite=Lax**
- refresh tokens are **never exposed to JavaScript**
- CSRF tokens protect state-changing requests

In production deployments cookies should always be served over **HTTPS**.

---

# Summary

Authara uses three cookies:

| Cookie | Purpose |
|------|------|
| `authara_access` | Short-lived JWT access token |
| `authara_refresh` | Refresh token for session continuation |
| `authara_csrf` | CSRF protection token |

This model allows secure browser-based authentication without exposing sensitive tokens to JavaScript.
