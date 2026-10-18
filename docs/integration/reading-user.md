# Reading the Authenticated User

Applications often need information about the currently authenticated user.

Examples include:

- displaying the user profile
- checking roles
- attaching the user ID to application data

Authara provides this information through the authenticated session.

---

# Reading authentication facts with the Go SDK

The Go SDK middleware validates the access token and adds stable authentication
facts to the request context:

```go
r.Use(sdk.RequireAuth)
```

```go
userID, ok := authara.UserIDFromContext(r.Context())
roles, _ := authara.RolesFromContext(r.Context())
organizationID, _ := authara.OrganizationIDFromContext(r.Context())
organizationRole, _ := authara.OrganizationRoleFromContext(r.Context())
```

Use these context values for identity facts needed on a hot request path. The
SDK deliberately does not turn them into application authorization policy.

When the handler needs the current profile rather than token facts, call the
generated client and forward the incoming request:

```go
client := authara.NewClient("http://authara:8080")
user, err := client.CallGetCurrentUser(r.Context(), r)
```

The browser SDK exposes the same endpoint through its generated client:

```ts
const user = await authara.getCurrentUser();
```

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

Native clients can use a bearer token instead:

```http
GET /auth/api/v1/user
Authorization: Bearer <access-token>
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
  "email_verified": true,
  "email_verified_at": "2026-01-01T11:30:00Z",
  "username": "user",
  "roles": [],
  "organization": {
    "id": "68c673e7-1ff9-4113-8bbf-e00f039a9a61",
    "name": "user",
    "role": "owner"
  },
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
- read the active organization ID and role
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
