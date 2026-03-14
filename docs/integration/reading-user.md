# Reading the Authenticated User

Applications often need information about the currently authenticated user.

Examples include:

- displaying the user profile
- checking roles
- attaching the user ID to application data

Authara provides this information through the authenticated session.

---

# Using the SDK

The recommended way to retrieve the authenticated user is through the Authara SDK.

Example using `authara-go`:

```go
user, err := authara.GetUser(r)

if err != nil {
    // user is not authenticated
}
```

If authentication succeeds, the SDK returns the authenticated user.

Example user object:

```
{
  id
  email
  username
  roles
  disabled
  created_at
}
```

---

# Using the HTTP API

Applications can also retrieve the user directly through the Authara API.

```
GET /auth/api/v1/user
```

Example request:

```
GET /auth/api/v1/user
Cookie: authara_access=<token>
```

---

# Successful response

```
200 OK
```

Example response:

```json
{
  "id": "a8f7c1f5-5d2b-4a3a-91c5-1c87b6e19c41",
  "email": "user@example.com",
  "username": "user",
  "roles": [],
  "disabled": false,
  "created_at": "2026-01-01T12:00:00Z"
}
```

---

# Unauthenticated requests

If the request does not contain a valid session:

```
401 Unauthorized
```

See:

- [API Errors](../api/errors.md)

---

# Typical usage

Applications typically read the user to:

- determine whether the user is authenticated
- retrieve user information
- check roles for authorization decisions

Example:

```
GET /auth/api/v1/user
```

If the request succeeds, the user is authenticated.

If the request returns `401`, the user must log in.

---

# Authorization

Authara provides **authentication**, not authorization.

Applications remain responsible for:

- deciding which users may access resources
- enforcing role-based access control
- implementing business logic

Authara only provides **the authenticated user identity**.

---

# Summary

To retrieve the authenticated user:

- use the SDK helper (recommended)
- or call `GET /auth/api/v1/user`

The endpoint returns user information if authenticated, otherwise `401`.
