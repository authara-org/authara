# OAuth Configuration

Authara supports optional OAuth authentication providers.

Currently supported providers:

```
google
```

Additional providers may be added in future versions.

---

See also: [Configuration Reference](reference.md)

---

## Enable providers

```
AUTHARA_OAUTH_PROVIDERS
```

Example:

```
AUTHARA_OAUTH_PROVIDERS=google
```

Multiple providers may be enabled using a comma-separated list.

---

## Google OAuth

Required configuration:

```
AUTHARA_OAUTH_GOOGLE_CLIENT_ID
```

Example:

```
AUTHARA_OAUTH_GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
```

The OAuth client must be configured with the correct redirect URL:

```
https://your-domain/auth/oauth/google/callback
```
