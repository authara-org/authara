# Authara Contract

This document defines the **public compatibility contract** of Authara.

Its purpose is to make upgrades safe for:

- applications integrating Authara
- SDKs built on top of Authara
- operators deploying Authara in production

If behavior described here changes incompatibly, that change is considered **breaking** and must follow the versioning policy below.

---

# 1. Scope

Authara exposes multiple public contracts:

- HTTP contract (routes, cookies, redirects, JSON behavior)
- configuration contract (environment variables)
- webhook contract (event types, payloads, delivery behavior)

This document defines the rules and guarantees that apply to all public contracts.

Machine-readable contract definitions live in the `contract/` directory.

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
- changing webhook payload shape or event names

## Minor release (`x.Y.z`)
Minor releases may introduce **additive** features.

Allowed in a minor release:

- new endpoints
- new optional request fields
- new optional response fields
- new cookies only if existing cookies remain unchanged
- new OAuth providers
- new webhook event types
- new non-breaking SDK integration capabilities

Not allowed in a minor release:

- removing or renaming stable public routes
- removing stable fields or cookies
- changing stable semantics incompatibly
- removing webhook event types

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

Unless explicitly stated otherwise, documented public behavior is considered **Stable**.

---

# 4. HTTP Contract

## 4.1 Stable routes

### Authentication UI / actions

- `GET /auth/login`
- `POST /auth/login`
- `GET /auth/signup`
- `POST /auth/signup`

### Session actions

- `POST /auth/sessions/logout`

### OAuth flow endpoints

Any documented OAuth callback endpoint used by browser-based integrations is part of the public contract once released and documented.

## 4.2 Method stability

For stable public endpoints:

- paths must remain unchanged
- HTTP methods must remain unchanged
- behavior must remain compatible

Changing methods or paths is a breaking change.

## 4.3 Status codes

Consumers may rely on the meaning of important status codes.

Precise HTML content is not stable unless explicitly documented.

---

# 5. Redirect Contract

Redirect behavior is part of the public contract.

## Browser redirects
Used for login, signup, logout, and protected flows.

## HTMX redirects
Use of `HX-Redirect` is stable once documented.

---

# 6. Cookie Contract

Stable cookies:

- `authara_access`
- `authara_refresh`
- `authara_csrf`

## Rules

- names must not change
- roles must remain consistent
- breaking security semantics is not allowed

---

# 7. JSON Contract

## Stable fields
Field names must not change or be removed.

## Error envelope

```json
{
  "error": {
    "code": "unauthorized",
    "message": "..."
  }
}
```

## Stable error codes

- `unauthorized`
- `forbidden`
- `invalid_request`
- `not_found`
- `internal_error`

---

# 8. Authentication Semantics

Stable behaviors include:

- access + refresh token model
- session lifecycle
- logout invalidation
- CSRF protection for browser flows

Security-relevant guarantees must not be weakened.

---

# 9. Webhook Contract

If webhooks are configured, the following are part of the public contract:

- webhook event types
- webhook payload envelope
- webhook header names
- signature format
- delivery semantics

Stable webhook guarantees:

- event type names must not change
- payload fields must not be removed or renamed
- headers must remain consistent
- signature format must remain compatible

Additive changes (e.g. new fields or events) are allowed in minor releases.

The machine-readable webhook contract is defined in:

```
contract/webhooks.yaml
```

---

# 10. Configuration Contract

Authara exposes configuration via environment variables.

Stable configuration variables are part of the public contract and follow the same versioning rules.

The machine-readable configuration contract is defined in:

```
contract/config.yaml
```

---

# 11. What Is Not Stable

Not part of the contract:

- internal code structure
- database schema
- HTML/CSS details
- internal APIs
- logging format
- internal error messages

---

# 12. Compatibility Testing

Authara maintains contract tests for:

- routes and methods
- cookies
- JSON structure and error codes
- redirect behavior
- configuration surface
- webhook event types and payload shape
- webhook signature format

---

# 13. Release Gate

Before releasing:

> Could an existing integration break without changes?

If yes → breaking change → must not ship as patch.

---

# 14. Practical Rule

If consumers can reasonably depend on a behavior, it is part of the contract.

When in doubt, preserve compatibility.
