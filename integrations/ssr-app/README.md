# Authara SSR app

This integration is a small server-rendered Go application behind the Authara
gateway. Open it through the gateway so `/auth/*` is routed to Authara,
`/spa/*` is routed to the React example, and the remaining paths reach this
app.

Authara directly owns its browser flows: login, signup, account management,
logout, and invitation acceptance. The app renders `/private`, validates the
session with `authara-go`, reads public Authara APIs on the user's behalf, and
uses the internal API only for server-side organization operations. The
internal API token is never sent to the browser.

From the repository root, start the development stack with:

```sh
make dev
```

Then visit `http://localhost:3001`. The private page is available at
`http://localhost:3001/private`, while the SPA is available at
`http://localhost:3001/spa/` without switching environments.

To check this module independently:

```sh
cd integrations/ssr-app
go test ./...
go vet ./...
go build ./cmd/ssr-app
```
