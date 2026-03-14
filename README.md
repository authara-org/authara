# Authara

**Authara is a self-hosted authentication service** for applications that want login, signup, sessions, and OAuth without outsourcing authentication to a SaaS provider.

Authara runs as a **separate server** alongside your application and provides the infrastructure required to manage authentication securely.

It is designed for:

- small teams and indie developers
- SSR, HTMX, and traditional backend applications
- self-hosted deployments
- plug-and-play integration with sensible defaults
- optional customization where needed

Authara is **not a library embedded into your application**.

It is **an authentication server that runs next to your app**.

---

# What Authara does

Authara provides the authentication infrastructure that applications would otherwise need to implement themselves.

Features include:

- built-in login and signup flows
- cookie-based sessions
- refresh token rotation and reuse detection
- OAuth provider login
- CSRF protection for browser flows
- a thin integration surface for applications and SDKs
- a stable HTTP contract for core routes and cookies

The goal is simple:

> Run Authara next to your app, mount it under `/auth`, and keep authentication infrastructure out of your application code.

More features are planned as the project evolves. See the roadmap for upcoming improvements.

---

# How it works

Authara is typically deployed together with a reverse proxy that routes requests between your application and Authara.

The system usually consists of three components:

1. **Authara** — the authentication server  
2. **Authara Gateway** — a lightweight reverse proxy  
3. **SDKs (optional)** — thin helpers for consuming authentication state

The gateway and SDKs are maintained in separate repositories.

---

## Default flow

1. A user requests a protected page in your application  
2. The application determines the user is not authenticated  
3. The application redirects the user to `/auth/login`  
4. Authara serves the login interface  
5. The user logs in  
6. Authara creates a session and sets authentication cookies  
7. The user is redirected back to the application  
8. The application continues handling the request with the authenticated session

This means the application developer:

- does not build login pages
- does not implement password storage
- does not implement refresh token rotation
- does not build authentication session infrastructure from scratch

Authara handles these concerns centrally.

---

# Deployment model

A typical Authara deployment consists of:

- **Authara** — the Go HTTP authentication server
- **PostgreSQL** — persistent storage
- **a reverse proxy or gateway** — routing requests

Typical routing:

```
/auth/* → Authara
/*      → Application
```

Authara is mounted under `/auth`, keeping authentication endpoints separate from application routes.

---

# Core principles

Authara is built around a few strict rules:

- **Clear boundaries** between configuration, HTTP handling, business logic, and persistence
- **Explicit transactions** owned by services, not HTTP handlers
- **No hidden magic**
- **No implicit database access**
- **Predictable infrastructure behavior**

Authara is infrastructure software. It should be understandable, operable, and safe.

---

# Sessions and tokens

Authara uses a two-token session model.

## Access token

- short-lived
- JWT-based
- stored in an HTTP-only cookie
- used for request authentication

## Refresh token

- opaque random token
- stored hashed in the database
- rotated on refresh
- reuse is treated as a security event

This model allows:

- stateless request authentication
- server-controlled session lifecycle

---

# CSRF protection

Authara protects state-changing browser requests with a CSRF mechanism:

- CSRF token stored in a cookie
- token required for protected POST requests
- HTML auth flows automatically include CSRF tokens
- browser authentication flows remain explicit and predictable

---

# Roles

Authara currently implements a minimal role model.

- built-in `admin` role
- regular users are simply non-admin users
- roles are facts carried in sessions and tokens

Authorization decisions remain the responsibility of the application.

Authara focuses strictly on authentication infrastructure.

---

# API and contract stability

Authara maintains a public HTTP contract for important behavior.

This includes:

- stable `/auth` browser routes
- stable `/auth/api/v1` JSON routes
- stable cookie names
- stable error envelope format
- contract tests for critical behavior

The goal is to make integrations predictable across releases.

---

# Migrations

Authara does **not automatically migrate the database schema**.

Schema changes are applied through explicit migrations.

Authara checks the database schema version during startup and fails fast if the running server is incompatible with the database.

This keeps upgrades deterministic and operator-controlled.

---

# What Authara is not

Authara is **not**:

- an enterprise IAM platform like Keycloak
- a hosted SaaS like Auth0 or WorkOS
- a framework embedded deeply into your application
- a general-purpose authorization engine

Authara is **self-hosted authentication infrastructure for applications**.

---

# Status

Authara is currently under active development.

Current priorities include:

- stable session infrastructure
- clear architecture boundaries
- predictable deployment behavior
- explicit authentication contracts
- contract stability

See the roadmap for upcoming features.

---

# Documentation

Full documentation is available at:

https://authara.org

---

# Summary

Authara is a **self-hosted authentication server** that provides login flows, sessions, and OAuth while keeping authentication infrastructure separate from your application.
