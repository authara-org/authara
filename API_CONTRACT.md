# API Contract

This document defines the **public compatibility contract** of Authara.

Its purpose is to make upgrades safe for:

- applications integrating Authara
- SDKs built on top of Authara
- operators deploying Authara in production

If behavior described here changes incompatibly, that change is considered **breaking** and must follow the versioning policy below.

---

# 1. Scope

This contract applies to the **externally observable behavior** of Authara.

It includes:

- public HTTP routes
- supported HTTP methods
- cookie names and their meaning
- redirect behavior relied upon by apps and SDKs
- stable JSON response shapes and error codes
- public authentication/session semantics

It does **not** include:

- internal package structure
- internal database schema
- internal Go APIs
- implementation details
- HTML/CSS markup details unless explicitly documented as stable

---

# 2. Versioning Policy

Authara follows semantic versioning for its public contract.

## Patch release (`x.y.Z`)
Patch releases must **not** break existing integrations.

Allowed in a patch release:

- bug fixes
- security fixes
- performance improvements
- internal refactors
- stricter validation only if it fixes clearly invalid behavior and does not break documented valid usage
- additive non-breaking response fields where consumers can safely ignore them

Not allowed in a patch release:

- removing or renaming public routes
- changing supported HTTP methods for public routes
- renaming cookies
- changing required request fields
- changing stable JSON field names
- changing stable error codes
- changing redirect semantics relied upon by apps or SDKs

## Minor release (`x.Y.z`)
Minor releases may introduce **additive** features.

Allowed in a minor release:

- new endpoints
- new optional request fields
- new optional response fields
- new cookies only if existing cookies remain unchanged
- new OAuth providers
- new non-breaking SDK integration capabilities

Not allowed in a minor release:

- removing or renaming stable public routes
- removing stable fields or cookies
- changing stable semantics incompatibly

## Major release (`X.y.z`)
Major releases may include breaking changes.

All breaking changes must be documented clearly in release notes and migration guides.

---

# 3. Stability Levels

Authara uses the following stability levels:

## Stable
Behavior is part of the public contract and may not break except in a major release.

## Deprecated
Behavior still works, but is scheduled for removal in a future major release.
Deprecated behavior must be documented before removal.

## Internal
Not part of the public contract.
May change at any time without notice.

Unless explicitly stated otherwise, public routes and cookies listed in this document are considered **Stable**.

---

# 4. Public HTTP Contract

## 4.1 Stable route contract

The following routes are part of the stable public contract.

### Authentication UI / actions

- `GET /auth/login`
- `POST /auth/login`
- `GET /auth/signup`
- `POST /auth/signup`

### Session actions

- `POST /auth/sessions/logout`

### OAuth flow endpoints

Any documented OAuth callback endpoint used by browser-based integrations is part of the public contract once released and documented.

If a route is listed as public in the route manifest or official documentation, it is considered stable unless explicitly marked otherwise.

## 4.2 Method stability

For stable public endpoints:

- the path must continue to exist
- the supported HTTP method must remain unchanged
- behavior must remain compatible

Changing:

- `GET` to `POST`
- `POST` to `PUT`
- route path names
- route nesting

is a breaking change.

## 4.3 Status code stability

Consumers may rely on the class and meaning of important status codes.

Examples:

- authentication-required routes must continue to signal unauthenticated access consistently
- forbidden admin access must continue to signal forbidden access consistently
- public form endpoints must continue to use the documented redirect behavior

Precise HTML content is not stable unless explicitly documented.

---

# 5. Redirect Contract

Redirect behavior is part of the public contract because Authara is designed to integrate with SSR/HTMX-style applications.

## 5.1 Browser redirects

For browser flows, Authara may use HTTP redirects for navigation.

The meaning of redirects for login, signup, logout, and protected-page flows is part of the stable contract.

## 5.2 HTMX redirects

Where Authara uses the `HX-Redirect` response header for HTMX-aware flows, that behavior is considered stable once released.

Applications and SDKs may rely on:

- the existence of redirect behavior for authenticated/unauthenticated flow transitions
- HTMX-safe redirect handling where documented

Changing HTMX redirect behavior incompatibly is a breaking change.

---

# 6. Cookie Contract

The following cookie names are stable public contract:

- `authara_access`
- `authara_refresh`
- `authara_csrf`

## 6.1 Stable cookie identity

These cookie names must not change except in a major release.

## 6.2 Stable cookie roles

### `authara_access`
Used for the short-lived access token.

### `authara_refresh`
Used for the refresh token / session continuation flow.

### `authara_csrf`
Used for CSRF protection.

## 6.3 Cookie attributes

The exact operational attributes may vary by environment where documented, for example:

- `Secure` in production
- `SameSite` policy
- `HttpOnly` depending on cookie purpose

However, changes that break documented security or integration semantics are breaking.

Examples of breaking cookie changes:

- renaming cookies
- changing the CSRF cookie to a different purpose without compatibility
- removing refresh cookie behavior relied upon by browser flows

---

# 7. JSON Response Contract

Where Authara exposes JSON endpoints, the following rules apply.

## 7.1 Stable field names

Stable JSON field names must not be renamed or removed except in a major release.

Adding optional fields is allowed in minor releases.

## 7.2 Error envelope

If Authara returns structured JSON errors, the envelope shape is part of the contract once documented.

Example stable structure:

{
  "error": {
    "code": "unauthorized",
    "message": "..."
  }
}

The `message` text may evolve for clarity.
The `code` value is contractually more important and should remain stable.

## 7.3 Stable error codes

Stable public JSON error codes include:

- `unauthorized`
- `forbidden`
- `invalid_request`
- `not_found`
- `internal_error`

Removing or renaming a stable error code is a breaking change.

---

# 8. Authentication and Session Semantics

The following behaviors are part of the public contract.

## 8.1 Access + refresh model

Authara uses a short-lived access token and a refresh token/session continuation model.

Applications and SDKs may rely on the existence of:

- access token validation semantics
- refresh-based session continuation semantics
- logout invalidation semantics

## 8.2 Refresh token reuse detection

If refresh token reuse detection is part of documented Authara behavior, weakening or removing that behavior is a breaking security change.

## 8.3 Audience semantics

If Authara documents supported token audiences such as app/admin semantics, those meanings are part of the contract.

## 8.4 CSRF enforcement

For browser state-changing flows under Authara-controlled routes, CSRF protection behavior is part of the security contract.

Changes that remove or weaken documented CSRF guarantees are breaking security changes.

---

# 9. What Is Not Stable

The following are **not** stable unless explicitly documented otherwise:

- internal HTML structure
- CSS classes
- DOM structure
- internal template names
- internal Go package APIs
- database schema details
- internal migration layout
- internal logging format
- internal error messages
- exact text copy in HTML pages

Consumers must not depend on internal implementation details.

---

# 10. Deprecation Policy

Stable public behavior may be deprecated before removal.

Deprecation process:

1. behavior is marked deprecated in docs and release notes
2. replacement is documented
3. deprecation remains available until the next major release unless otherwise stated for urgent security reasons

Patch releases must not silently remove stable public behavior.

---

# 11. Compatibility Testing

Authara should maintain contract tests for:

- stable routes
- stable HTTP methods
- stable cookie names
- stable JSON error envelope / codes
- stable redirect semantics for documented flows

SDKs should also maintain integration tests against supported Authara versions where practical.

---

# 12. Release Gate

Before any Authara release, maintainers should ask:

Could an existing integration or SDK that worked on the previous release fail without code changes after upgrading to this release?

If the answer is yes, the change is breaking and must not ship as a patch release.

---

# 13. Practical Rule

If applications or SDKs can reasonably depend on a behavior, that behavior should be treated as public contract unless clearly documented as internal.

When in doubt, preserve compatibility.
