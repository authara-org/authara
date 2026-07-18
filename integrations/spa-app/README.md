# Authara SPA integration

A minimal React application that uses Authara's public browser API directly. It has no application backend and deliberately delegates login, signup, and account management to Authara's hosted pages.

## Run with the repository

From the repository root, start the complete development stack:

```sh
make dev
```

Open the SPA through the gateway at
[http://localhost:3001/spa/](http://localhost:3001/spa/). The SSR example remains
available at [http://localhost:3001](http://localhost:3001), and Authara owns
`/auth/*` on that same origin. Vite runs inside Compose, so React source changes
are applied without rebuilding or restarting the stack.

For production, build the Docker image and serve it at `/spa/` behind an Authara
Gateway. The nginx configuration serves `index.html` for client-side paths such
as `/spa/private`; the gateway must continue routing `/auth/*` to Authara.

## Included behavior

- Authara-hosted login and signup, returning to `/spa/private`
- cookie-session refresh followed by a single retry
- current user, organization memberships, active organization, member list, and
  the signed-in user's member detail through the public organization API
- organization creation and renaming when allowed by the configured mode
- invitation listing, detail lookup, creation, revocation, and resending
- active-organization switching, API logout, and a link to Authara's account page
- graceful unavailable states when the current role cannot see members or manage
  invitations

All requests use relative URLs. API mutations fetch `/auth/api/v1/csrf` first and send its value as `X-CSRF-Token`; access and refresh tokens stay in Authara's cookies and returned token strings are ignored.

Direct organization management is opt-in through
`AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED`. The SPA never receives the
internal API token; Authara derives the actor and current organization from the
browser session.
