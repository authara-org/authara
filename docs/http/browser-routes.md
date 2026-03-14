# Browser Routes

Authara provides built-in browser routes that implement common authentication flows such as login, signup, and logout.

These routes return **HTML pages** and are intended to be accessed directly by a web browser.

All browser routes are served under the `/auth` base path.

Example:

```
/auth/login
/auth/signup
/auth/logout
```

Applications typically redirect users to these routes when authentication is required.

---

# Login

```
GET /auth/login
```

Displays the login page.

The login page allows users to authenticate using:

- email and password
- configured OAuth providers (if enabled)

### Query Parameters

| Parameter | Description |
|----------|-------------|
| `return_to` | Path the user should be redirected to after successful authentication |

Example:

```
/auth/login?return_to=/dashboard
```

If the `return_to` parameter is provided, Authara redirects the user to that path after successful login.

If the parameter is not provided, Authara redirects to:

```
/
```

### Security

For security reasons, Authara only allows **relative paths** in `return_to`.

External URLs are rejected to prevent open redirect vulnerabilities.

---

# Signup

```
GET /auth/signup
```

Displays the signup page.

The signup page allows new users to create an account.

After successful signup, Authara creates a session and redirects the user according to the `return_to` parameter if present.

Example:

```
/auth/signup?return_to=/welcome
```

If `return_to` is not provided, the user is redirected to:

```
/
```

---

# Logout

```
POST /auth/logout
```

Logs out the current user session.

The logout request requires CSRF protection.

### Required Header

```
X-CSRF-Token: <csrf-token>
```

The token must match the value stored in the `authara_csrf` cookie.

When using HTML forms, the CSRF token may also be submitted as a **hidden form field**.

Example:

```html
<input type="hidden" name="csrf_token" value="...">
```

See the [CSRF documentation](../api/cookies.md) for details.

### Example

```
POST /auth/logout
X-CSRF-Token: <csrf-token>
```

After logout, Authara clears the authentication cookies and redirects the user to:

```
/
```

---

# OAuth Login

If OAuth providers are configured, the login page may offer buttons for external login providers.

Example providers:

- Google
- GitHub (future)
- Microsoft (future)

The OAuth flow is handled entirely by Authara.

After successful authentication with the provider, the user is redirected back to Authara and a session is created.

The final redirect follows the same `return_to` behavior as the normal login flow.

---

# Summary

Browser routes provide the user-facing authentication flows used by applications.

Typical usage pattern:

1. The application detects an unauthenticated user
2. The user is redirected to `/auth/login`
3. The user completes authentication
4. Authara creates a session
5. The user is redirected back to the application
