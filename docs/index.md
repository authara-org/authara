# Authara

Authara is a **self-hosted authentication service** for applications that want login, sessions, and OAuth without outsourcing authentication to a SaaS provider.

Authara runs as a **separate server** alongside your application and provides the infrastructure required to manage authentication securely.

Authara is designed for:

- small teams and indie developers
- any type of web application (SSR, SPA, HTMX, or traditional backends)
- self-hosted deployments
- predictable authentication infrastructure

Authara is **not a library embedded into your application**.

It is **an authentication server that runs next to your app**.

---

## What Authara provides

Authara implements the authentication infrastructure that applications would otherwise need to build themselves.

Features include:

- login and signup flows
- cookie-based sessions
- refresh token rotation
- Google OAuth login
- CSRF protection for browser flows
- a stable HTTP authentication interface
- event webhooks for user lifecycle integration (e.g. account creation and deletion)

Your application remains responsible for:

- authorization decisions
- business logic
- application data

Authara focuses strictly on **authentication and session management**.

---

## SDKs

Authara can be integrated directly via its HTTP interface, but optional SDKs are available to simplify common integration tasks.

Available SDKs include:

- **authara-go** – Go server-side integration
- **@authara/browser** – browser utilities for CSRF, session refresh, and logout

SDKs are intentionally thin and do not hide Authara's HTTP interface.  
They are convenience helpers only.

Applications remain free to interact with Authara directly via HTTP if desired.

---

## How Authara works

Authara is typically deployed behind a reverse proxy such as the **Authara Gateway** and mounted under `/auth`.

Example routing:

```
/auth/* → Authara
/*      → your application
```

A typical authentication flow looks like this:

1. A user requests a protected page in your application
2. The application detects the user is not authenticated
3. The application redirects the user to `/auth/login`

The redirect may include a `return_to` query parameter that specifies where the user should be redirected after successful authentication.

Example:

```
/auth/login?return_to=/dashboard
```

4. Authara serves the login interface
5. The user logs in
6. Authara creates a session and sets authentication cookies
7. The user is redirected back to the value of `return_to` (or `/` if not provided)

With Authara, application developers do **not** need to implement:

- password storage
- session infrastructure
- refresh token rotation
- OAuth login flows

These concerns are handled centrally by Authara.

---

### `return_to` parameter

The `return_to` query parameter controls the redirect target after a successful login or signup.

Example:

```
/auth/login?return_to=/settings
```

If the parameter is not provided, Authara redirects to `/`.

For security reasons, Authara only allows **relative paths** to prevent open redirect vulnerabilities.

---

## Architecture

Authara follows a layered architecture with explicit boundaries between components.

```
HTTP layer
   ↓
Services (auth, session)
   ↓
Store (database access)
   ↓
PostgreSQL
```

Key principles:

- explicit transactions
- predictable session behavior
- no hidden database access
- clear separation of responsibilities

Authara is designed to behave like **infrastructure**, not application code.

---

## Deployment model

A typical Authara deployment consists of:

- **Authara server** – the authentication service
- **PostgreSQL** – required database
- **reverse proxy or gateway** – routes requests

Example deployment:

```
User
 ↓
Authara Gateway
 ├── /auth/* → Authara
 └── /*      → Application
```

Authara supports being mounted under a base path such as `/auth`.

---

## Quick start

To get started quickly:

1. Run Authara with PostgreSQL
2. Start the Authara Gateway
3. Mount Authara under `/auth`
4. Redirect unauthenticated users to `/auth/login`

Continue with the [Quickstart](quickstart.md) guide.

---

## Summary

Authara is a **self-hosted authentication server** that provides login flows, sessions, and OAuth while keeping authentication infrastructure separate from your application.
