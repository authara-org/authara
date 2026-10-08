# Authara SPA integration

A React application that uses Authara's public browser API directly wherever a
public route exists. It implements its own password, Google, and Apple login,
provider-collision recovery, passkey login, signup, signup verification,
account-management, and organization screens. It never embeds or navigates to
Authara's hosted user-account page.

Operations that intentionally exist only on Authara's internal API go through
the example application's Go backend at `/spa/api/v1`. That backend resolves
the actor from the Authara session, validates CSRF, applies Authara's
recent-authentication policy, and holds the internal API token. React never
receives that token and cannot choose the internal actor.

## Run with the repository

From the repository root, start the complete development stack:

```sh
make dev
```

Open the SPA through the gateway at
[http://localhost:3001/spa/](http://localhost:3001/spa/). The SSR example remains
available at [http://localhost:3001](http://localhost:3001), and Authara owns
`/auth/*` on that same origin. The React account screen is available at
`/spa/account`. Vite runs inside Compose, so React source changes are applied
without rebuilding or restarting the stack.

To show the Google button, set:

```sh
AUTHARA_OAUTH_PROVIDERS=google
AUTHARA_OAUTH_GOOGLE_CLIENT_ID=your-google-client-id.apps.googleusercontent.com
```

Add the gateway origin (for example `http://localhost:3001`) to the Google OAuth
client's authorized JavaScript origins. The SPA does not load Google's script
when the provider is disabled.

To show the Apple button, enable Apple and configure its Services ID, team ID,
key ID, private key, and redirect URL as described in the OAuth configuration
documentation. The SPA uses Apple's popup flow and exchanges only the
single-use authorization code with Authara.

For production, build the Docker image and serve it at `/spa/` behind an Authara
Gateway. The nginx configuration serves `index.html` for client-side paths such
as `/spa/private`; the gateway must continue routing `/auth/*` to Authara. A
deployment that enables the internal-only controls must also route
`/spa/api/v1/*` to an application backend equivalent to the example Go backend.

## Included behavior

- custom password login and signup through `/auth/api/v1`
- custom passkey login and registration through the WebAuthn API
- invitation-code signup through the same public signup API
- custom Google login through the Google Identity Services button and Authara's
  nonce-bound `/auth/api/v1/oauth/google` flow when Google is enabled
- custom Apple login through the server-created state and nonce flow
- account-collision recovery that displays the existing account's available
  password, Google, or Apple proof methods and links the attempted Google
  identity after successful proof
- signup email-code verification and opaque challenge resending
- cookie-session refresh followed by a single retry
- current user, organization memberships, active organization, member list, and
  the signed-in user's member detail through the public organization API
- organization creation, renaming, deletion, member removal/leaving, and
  ownership transfer when allowed by the configured mode and role
- invitation listing, detail lookup, role-aware creation, revocation, and
  resending
- React-owned profile, email, password, Google, passkey, and session management
- current-user deletion through the application backend and Authara internal API
- recent-authentication handling that preserves the `428` challenge, opens a
  React dialog, and retries the original mutation after successful step-up
- active-organization switching and API logout
- graceful unavailable states when the current role cannot see members or manage
  invitations

All requests use relative URLs. API and backend mutations fetch
`/auth/api/v1/csrf` first and send its value as `X-CSRF-Token`; access and
refresh tokens stay in Authara's cookies and returned token strings are ignored.

To exercise signup verification locally, run `make mailpit-up`, use the SMTP
development configuration, and read the code at
[http://localhost:8025](http://localhost:8025). With the `noop` email provider,
disable challenges to test immediate signup because verification codes are not
exposed.

Direct organization management is opt-in through
`AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED`. Public reads, renames, and
invitation revocation go directly to Authara. The application backend uses the
internal API only for operations without a public route. It derives the actor
from `/auth/api/v1/user` and never accepts an actor ID from React.

Authara's `/auth/admin/*` and `/auth/operator/*` workspaces intentionally remain
server-rendered. This example owns only the end-user SPA experience.
