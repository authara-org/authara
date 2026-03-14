# Components

Authara consists of several components that together provide authentication infrastructure for applications.

The system is designed to be modular so each component can evolve independently.

---

# Authara Core

**Authara Core** is the main authentication server.

Most of the documentation on this site refers to **Authara Core** and its HTTP interface.

Authara Core provides:

- login and signup flows
- session management
- refresh token rotation
- OAuth login
- authentication APIs

Authara Core is implemented as a Go HTTP server backed by PostgreSQL.

Repository:

https://github.com/authara-org/authara

---

# Authara Gateway

The **Authara Gateway** is a lightweight reverse proxy that routes requests between your application and Authara.

Typical routing:

```
/auth/* → Authara
/*      → Application
```

The gateway simplifies deployments and provides a consistent entry point for authentication routes.

Repository:

https://github.com/authara-org/authara-gateway

---

# SDKs

Authara provides optional SDKs that simplify integration with applications.

SDKs are intentionally thin and wrap the public HTTP API.

Available SDKs include:

- **authara-go** — Go integration helpers
- **@authara/browser** — browser helpers for CSRF, refresh, and logout

Applications may always call the HTTP API directly if preferred.

---

# Summary

Authara is composed of several small components:

| Component | Purpose |
|------|------|
| Authara Core | Authentication server |
| Authara Gateway | Reverse proxy routing `/auth` |
| Migrations | Database schema management |
| SDKs | Optional integration helpers |

Together these components provide a complete self-hosted authentication infrastructure.
