# OAuth Configuration

Authara supports optional OAuth authentication providers.

Currently supported providers:

```
google
apple
microsoft
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
AUTHARA_OAUTH_PROVIDERS=google,apple,microsoft
```

Multiple providers may be enabled using a comma-separated list.

## Browser and native clients

Browser clients use Authara's OAuth state cookies by default. Native clients
request each provider's options and login with `session_transport=token`; Core
then returns a short-lived `challenge_id` bound to the provider, state, and
nonce. The native client returns that ID with the provider proof and receives
Authara access and refresh tokens in JSON without browser cookies or CSRF
credentials.
See [API Endpoints](../api/endpoints.md#session-transports) for the transport
contract.

---

## Google OAuth

Required configuration:

```
AUTHARA_OAUTH_GOOGLE_CLIENT_ID
AUTHARA_OAUTH_GOOGLE_CLIENT_SECRET
```

Example:

```
AUTHARA_OAUTH_GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
AUTHARA_OAUTH_GOOGLE_CLIENT_SECRET=your-client-secret
```

Use a Web application OAuth client. Add every website that displays an Authara
Google button to the client's Authorized JavaScript origins. For example:

```
https://your-domain
```

Authara uses Google's popup authorization-code flow. The browser sends the
one-time code to Core, and Core exchanges it using the client secret. Keep the
client secret on the Authara server; never expose it to browser JavaScript.

## Microsoft identity platform

Create an app registration in Microsoft Entra and add this exact Single-page
application (SPA) redirect URI for every origin that displays an Authara
Microsoft button:

```
https://your-domain/auth/oauth/microsoft/redirect
```

Enable ID tokens for the application. The redirect page is Authara's dedicated
MSAL bridge and must keep the `/auth/oauth/microsoft/redirect` path.

Microsoft does not guarantee a verified email claim, including for some work
and school accounts. Authara therefore keys Microsoft identities by their
immutable tenant and object IDs and stores the returned email as unverified
unless Microsoft supplies `verified_primary_email` or a true `xms_edov` claim.
You may add both claims under **Token configuration** > **Add optional claim**
> **ID**; the browser already requests the `email` OpenID Connect scope that
`xms_edov` depends on.

Required configuration:

```dotenv
AUTHARA_OAUTH_PROVIDERS=microsoft
AUTHARA_OAUTH_MICROSOFT_CLIENT_ID=00000000-0000-0000-0000-000000000000
AUTHARA_OAUTH_MICROSOFT_TENANT_ID=common
```

`AUTHARA_OAUTH_MICROSOFT_TENANT_ID` accepts a tenant UUID or `common`,
`organizations`, or `consumers`. Use a tenant UUID for a single-tenant app,
`organizations` for work and school accounts, `consumers` for personal
Microsoft accounts, or `common` for both.

Authara's button uses Microsoft's popup authorization-code flow with PKCE.
Core validates the resulting v2 ID token against the issuing tenant, client
ID, signature, expiry, and one-time nonce. Accounts are keyed by the immutable
tenant ID and object ID, not by a mutable email address. If the suggested email
already belongs to an Authara account, Authara requires proof using an existing
sign-in method before linking Microsoft. An unverified Microsoft email never
marks the Authara email as verified. New accounts retain `email_verified=false`;
enable Authara's required email-verification policy if the application requires
mailbox ownership before access.

### Troubleshooting Microsoft sign-in

- A Microsoft address can belong to either a personal account or a work or
  school account. The account picker may ask which identity type to use.
- Missing Microsoft email-verification claims do not block sign-in or linking,
  but the email remains unverified until Authara verifies it separately.
- The redirect URI must be registered under the **Single-page application**
  platform and must exactly match the button origin plus
  `/auth/oauth/microsoft/redirect`.

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
# Optional for native iOS token sessions:
AUTHARA_OAUTH_APPLE_NATIVE_CLIENT_ID=com.example.app
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

Set `AUTHARA_OAUTH_APPLE_NATIVE_CLIENT_ID` to the native Apple App ID (the iOS
bundle identifier) when native clients send authorization codes to Authara.
Authara exchanges those token-session codes with the native client ID and no
web redirect URI. Browser sessions continue to use the Services ID and return
URL from `AUTHARA_OAUTH_APPLE_CLIENT_ID`. The same Apple team, Sign in with
Apple key, nonce, state, and one-time challenge protections apply to both.

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
- A user whose Apple email already belongs to an Authara account can prove
  ownership with an existing method and complete Authara's collision-recovery
  flow, or sign in first and link Apple from the account page.
