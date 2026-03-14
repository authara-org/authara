# Protecting Routes

Applications often need to restrict access to certain routes to authenticated users.

Authara does **not automatically enforce route protection** inside your application.  
Instead, the application checks whether a user is authenticated and redirects to the login page if necessary.

The recommended approach is to use the **Authara SDK middleware**.

---

# Middleware

The `authara-go` SDK provides middleware that validates the session for incoming requests.

Two variants are available:

- **RequireAuth** — strict authentication check
- **RequireAuthWithRefresh** — automatically refreshes sessions

---

# Strict authentication

The strict middleware validates the access token and returns `401` if the user is not authenticated.

Example:

```go
router.Use(authara.RequireAuth())
```

If the access token is invalid or expired:

```
401 Unauthorized
```

The application can then decide how to handle the response (for example redirecting to login).

This mode is useful for:

- APIs
- strict authentication boundaries
- applications that handle refresh logic separately

---

# Automatic refresh

The refresh middleware attempts to refresh the session if the access token has expired.

Example:

```go
router.Use(authara.RequireAuthWithRefresh())
```

Behavior:

1. Access token is validated
2. If expired, the SDK attempts a refresh
3. If refresh succeeds, the request continues
4. If refresh fails, the request returns `401`

This approach works well for:

- SSR applications
- traditional server-rendered apps
- applications that want seamless sessions

---

# Redirecting to login

If a request is unauthenticated, applications typically redirect the user to:

```
/auth/login
```

Including the original path:

```
/auth/login?return_to=/dashboard
```

This ensures the user returns to the original page after authentication.

See:

- [Login Redirect](login-redirect.md)

---

# Authorization

Authara provides **authentication**, not authorization.

Applications remain responsible for:

- deciding which routes require authentication
- enforcing role-based access
- implementing business logic

Authara only provides **the authenticated user identity**.

---

# Summary

To protect application routes:

1. Use the SDK middleware
2. Choose between strict or auto-refresh behavior
3. Redirect unauthenticated users to `/auth/login`
4. Include a `return_to` parameter
