# Login Redirect

Applications typically redirect users to Authara when authentication is required.

The login page is available at:

```
/auth/login
```

When a user is not authenticated, the application should redirect the user to this route.

---

# Basic redirect

Example redirect:

```
/auth/login
```

Authara will display the login page and handle the authentication flow.

---

# Returning to the original page

Applications usually want users to return to the page they originally requested.

Authara supports this using the `return_to` query parameter.

Example:

```
/auth/login?return_to=/dashboard
```

After successful authentication, Authara redirects the user to the provided path.

---

# Example flow

User requests:

```
/dashboard
```

Application redirects:

```
/auth/login?return_to=/dashboard
```

User logs in successfully.

Authara redirects:

```
/dashboard
```

---

# Security

For security reasons, Authara only accepts **relative paths** for the `return_to` parameter.

Valid examples:

```
/dashboard
/settings
/admin/users
```

Invalid examples:

```
https://example.com/dashboard
https://malicious-site.com
```

This prevents **open redirect vulnerabilities**.

---

# SDK helpers

Some Authara SDKs provide helpers for building login redirects.

For example, the `authara-go` SDK includes helpers for detecting unauthenticated requests and redirecting users to the login page.

See:

- [Protecting Routes](protecting-routes.md)

---

# Summary

To redirect users to login:

1. Detect that the user is not authenticated
2. Redirect to `/auth/login`
3. Include a `return_to` parameter
4. Authara handles the login flow and redirects back
