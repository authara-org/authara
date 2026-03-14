# Architecture Overview

Authara is designed as **authentication infrastructure**, not application code.

It runs as a **separate service** alongside your application and is responsible only for authentication concerns such as login, sessions, and OAuth.

Applications interact with Authara through a **stable HTTP interface**, while Authara manages the underlying authentication mechanisms and session lifecycle.

---

## System Overview

A typical Authara deployment consists of three components:

- **Authara Core** – the authentication server
- **PostgreSQL** – persistent storage
- **Gateway / reverse proxy** – routes traffic between Authara and the application

Example request flow:

```
Client
  ↓
Authara Gateway
  ├── /auth/* → Authara
  │                ↓
  │            PostgreSQL
  │
  └── /* → Application
```

The gateway isolates authentication endpoints under `/auth` while forwarding all other traffic to the application.

---

## Responsibilities

Authara handles **authentication infrastructure**, while the application handles **application logic**.

### Authara

Authara is responsible for:

- login and signup flows
- password verification
- OAuth provider integration
- session creation and validation
- refresh token rotation
- CSRF protection
- issuing access tokens

### Application

Applications remain responsible for:

- authorization decisions
- permissions and roles
- business logic
- application data

Authara intentionally **does not implement authorization policies**.

---

## Internal Structure

Authara follows a simple layered architecture:

```
HTTP
 ↓
Services
 ↓
Store
 ↓
PostgreSQL
```

Each layer has a clear responsibility:

- **HTTP layer** – request handling, routing, cookies, and responses
- **Services** – authentication logic and session lifecycle
- **Store** – database access
- **PostgreSQL** – persistent storage

This separation keeps authentication logic explicit and prevents hidden database access.

---

## Session Model

Authara uses a **two-token session model**.

### Access token

- short-lived JWT
- stored in an HTTP-only cookie
- used for request authentication

### Refresh token

- long-lived opaque token
- stored hashed in the database
- rotated on refresh
- used to obtain new access tokens

This provides **efficient request authentication** while maintaining **server-side session control**.

---

## Summary

Authara separates authentication infrastructure from application logic.

It provides a dedicated authentication service responsible for login flows, sessions, and OAuth while allowing applications to focus entirely on business logic and authorization.
