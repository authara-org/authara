# OAuth Configuration

Authara supports optional OAuth authentication providers.

Currently supported providers:

```
google
apple
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
AUTHARA_OAUTH_PROVIDERS=google,apple
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

## Sign in with Apple

Apple web sign-in requires an Apple Developer Program team, a primary App ID
with Sign in with Apple enabled, a Services ID associated with that App ID, and
a Sign in with Apple private key. The Services ID is the client ID.

Register this return URL for the Services ID:

```text
https://your-domain/auth/oauth/apple/callback
```

Apple requires a real HTTPS domain for web return URLs; `localhost` and IP
addresses are not supported. Use an HTTPS development domain, tunnel, or
staging deployment for end-to-end testing.

Enable Apple and configure all of these startup settings:

```dotenv
AUTHARA_OAUTH_PROVIDERS=apple
AUTHARA_OAUTH_APPLE_CLIENT_ID=com.example.web
AUTHARA_OAUTH_APPLE_TEAM_ID=ABCDE12345
AUTHARA_OAUTH_APPLE_KEY_ID=XYZ9876543
AUTHARA_OAUTH_APPLE_PRIVATE_KEY_BASE64=<base64-encoded-p8-file>
AUTHARA_OAUTH_APPLE_TOKEN_ACTIVE_KEY_ID=key-1
AUTHARA_OAUTH_APPLE_TOKEN_KEYS=key-1:<base64-encoded-32-byte-key>
```

The private key signs short-lived Apple client secrets. The separate 32-byte
token key encrypts Apple refresh tokens at rest so Authara can revoke Apple
authorization when a user unlinks Apple or deletes their account. Keep both
values in a secret manager. Generate a token key with `openssl rand -base64 32`.
Key IDs allow a later rotation while old ciphertext is still decryptable.

The hosted UI and React SPA use Apple's official browser SDK. Only the
single-use authorization code and state reach Authara; the Apple private key
and encrypted refresh token never enter browser code.

If users select Apple's private email relay, configure the sending domain with
Apple and ensure SPF and DKIM are valid before sending mail to relay addresses.

### Rotate Apple keys

To rotate the refresh-token encryption key, add the new key to
`AUTHARA_OAUTH_APPLE_TOKEN_KEYS`, change
`AUTHARA_OAUTH_APPLE_TOKEN_ACTIVE_KEY_ID` to its ID, and restart Authara. Keep
the previous key in the keyring while any active credential or queued token
revocation still uses its ID. New and refreshed credentials are written with
the active key. Remove an old key only after neither table references it:

```sql
SELECT encryption_key_id, sum(token_count) AS token_count
FROM (
  SELECT encryption_key_id, count(*) AS token_count
  FROM authara.apple_credentials
  GROUP BY encryption_key_id
  UNION ALL
  SELECT encryption_key_id, count(*) AS token_count
  FROM authara.apple_token_revocations
  GROUP BY encryption_key_id
) apple_tokens
GROUP BY encryption_key_id;
```

When an account is deleted or Apple is unlinked, Authara copies the encrypted
refresh token to a durable revocation queue before deleting the local identity.
The maintenance worker retries Apple outages with backoff; queued tokens remain
encrypted until Apple accepts their revocation.

To rotate Apple's `.p8` signing key, create and enable the replacement in the
Apple Developer portal, then deploy its key ID and base64-encoded private key
together. Authara-generated Apple client secrets live for five minutes, so do
not revoke the previous Apple key until that overlap has passed.

### Troubleshooting Apple sign-in

- If the Apple popup reports an invalid redirect URI, verify that the exact
  `PUBLIC_URL` domain and `/auth/oauth/apple/callback` return URL are registered
  for the Services ID. Scheme, host, port, and path must match.
- If Authara fails at startup, check that the `.p8` value is base64 encoded as
  one line and that every refresh-token key decodes to exactly 32 bytes.
- If sign-in works but relay mail is not delivered, verify the sending domain,
  SPF, and DKIM configuration in Apple's private email relay settings.
- A user whose Apple email already belongs to an Authara account must first
  sign in with the existing method and then link Apple from the account page.
