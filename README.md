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

## Quickstart

The fastest way to run Authara locally is with Docker.

A minimal Docker Compose setup looks like this:

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: authara
      POSTGRES_USER: authara
      POSTGRES_PASSWORD: authara

  app:
    image: nginx:alpine

  authara-migrations:
    image: ghcr.io/authara-org/authara-migrations:latest
    env_file:
      - .env
    depends_on:
      - postgres

  authara:
    image: ghcr.io/authara-org/authara-core:latest
    env_file:
      - .env
    depends_on:
      authara-migrations:
        condition: service_completed_successfully

  gateway:
    image: ghcr.io/authara-org/authara-gateway:latest
    ports:
      - "3000:3000"
    environment:
      GATEWAY_BIND: :3000
      AUTHARA_UPSTREAM: authara:8080
      APP_UPSTREAM: app:80
    depends_on:
      - authara
      - app
```

Create a `.env` file with your Authara configuration, then start the stack:

```bash
docker compose up
```

Once everything is running, open:

```text
http://localhost:3000/auth/login
```

Authara is typically mounted behind a gateway or reverse proxy:

```text
/auth/* → Authara
/*      → Your application
```

For the full Quickstart, configuration reference, and deployment details, see the documentation:

https://docs.authara.org/quickstart

---

# What Authara does

Authara provides the authentication infrastructure that your applications would otherwise need to implement themselves.

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

---

# How it works

Authara is typically deployed together with a reverse proxy that routes requests between your application and Authara.

The system usually consists of three components:

1. **Authara** — the authentication server  
2. **Authara Migrations** — migrations for PostgreSQL  
3. **Authara Gateway** — a lightweight reverse proxy  
4. **SDKs** — thin helpers for consuming authentication state

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

This means you:

- do not have to build login pages
- do not have to implement password storage
- do not have to implement refresh token rotation
- do not have to build authentication session infrastructure from scratch

Authara handles these concerns centrally.

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

Authara is **self-hosted authentication infrastructure for applications**.

---

# Documentation

Full documentation is available at:

https://docs.authara.org

---

# Examples

Example applications demonstrating how to integrate Authara with different technology stacks are available in a separate repository:

https://github.com/authara-org/authara-examples

These examples include minimal setups for different stacks and show how to integrate authentication end-to-end.

