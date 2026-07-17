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
- current user, organization memberships, active organization, and member list
- active-organization switching, API logout, and a link to Authara's account page
- a graceful unavailable state when the current organization mode hides members

All requests use relative URLs. API mutations fetch `/auth/api/v1/csrf` first and send its value as `X-CSRF-Token`; access and refresh tokens stay in Authara's cookies and returned token strings are ignored.

Organization creation, organization updates, invitations, and capability discovery are intentionally absent. Those routes currently belong to Authara's internal API and require a server-side bearer secret that must never be shipped to a browser bundle.
