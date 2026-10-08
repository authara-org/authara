// Package showcase renders Core's real UI templates with deterministic fixture
// data. It is registered only when APP_ENV=dev.
package showcase

import (
	"html/template"
	"net/http"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/render"
	templates "github.com/authara-org/authara/internal/http/templates"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	challengeview "github.com/authara-org/authara/internal/http/templates/challenge"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/http/viewmodel"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Page struct {
	Slug      string
	Name      string
	Group     string
	Component func() templ.Component
}

type Handler struct {
	render render.Renderer
	pages  []Page
	bySlug map[string]Page
}

func New(renderer render.Renderer) *Handler {
	pages := fixturePages()
	bySlug := make(map[string]Page, len(pages))
	for _, page := range pages {
		bySlug[page.Slug] = page
	}
	return &Handler{render: renderer, pages: pages, bySlug: bySlug}
}

func (h *Handler) Gallery(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := galleryTemplate.Execute(w, h.pages); err != nil {
		http.Error(w, "showcase template error", http.StatusInternalServerError)
	}
}

func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	page, ok := h.bySlug[chi.URLParam(r, "slug")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := h.render(w, r, http.StatusOK, page.Component()); err != nil {
		return
	}
}

func Pages() []Page {
	return append([]Page(nil), fixturePages()...)
}

func fixturePages() []Page {
	providers := []oauth.OAuthProvider{{Name: domain.ProviderGoogle, ClientID: "showcase-google-client"}}
	pages := []Page{
		{Slug: "login", Name: "Sign in", Group: "Authentication", Component: func() templ.Component { return authview.Login(providers, true, "Authara") }},
		{Slug: "signup", Name: "Sign up", Group: "Authentication", Component: func() templ.Component { return authview.Signup(providers, "Authara") }},
		{Slug: "password-reset", Name: "Password reset", Group: "Authentication", Component: authview.PasswordReset},
		{Slug: "verify-signup", Name: "Verify sign-up", Group: "Authentication", Component: func() templ.Component {
			return challengeview.VerifyChallenge("11111111-1111-1111-1111-111111111111", "signup", "Verify your email")
		}},
		{Slug: "verify-password-reset", Name: "Verify password reset", Group: "Authentication", Component: func() templ.Component {
			return challengeview.VerifyChallenge("22222222-2222-2222-2222-222222222222", "password-reset", "Reset your password")
		}},
		{Slug: "verify-email-change", Name: "Verify email change", Group: "Authentication", Component: func() templ.Component {
			return challengeview.VerifyChallenge("33333333-3333-3333-3333-333333333333", "email-change", "Verify your new email")
		}},
		{Slug: "successful-deletion", Name: "Successful account deletion", Group: "Authentication", Component: authview.SuccessfulDeletion},
		{Slug: "account-collision", Name: "Account collision", Group: "Authentication", Component: func() templ.Component {
			return authview.AccountCollision("33333333-3333-3333-3333-333333333333", "alex@example.com", "google", []authview.AccountCollisionProofOption{{Provider: "password", Label: "Confirm with password"}})
		}},
		{Slug: "invitation-accept", Name: "Invitation", Group: "Invitations", Component: func() templ.Component {
			return authview.InvitationAccept("Acme Studio", "alex@example.com", "alex@example.com", "Member", "Oct 4, 2026", "fixture-token", "Join Acme Studio", "Review the invitation before joining.", "", "Join organization", nil, false, providers)
		}},
		{Slug: "invitation-signup", Name: "Invitation sign-up", Group: "Invitations", Component: func() templ.Component {
			return authview.InvitationSignup("Acme Studio", "alex@example.com", "fixture-token", providers)
		}},
		{Slug: "invitation-login", Name: "Invitation login", Group: "Invitations", Component: func() templ.Component {
			return authview.InvitationLogin("Acme Studio", "alex@example.com", "fixture-token", providers)
		}},
		{Slug: "account", Name: "Account", Group: "Account", Component: fixtureAccount},
		{Slug: "passkey-setup", Name: "Passkey setup", Group: "Account", Component: func() templ.Component { return authview.PasskeySetup("/auth/account") }},
		{Slug: "add-password", Name: "Add password", Group: "Account", Component: authview.AddPassword},
		{Slug: "change-password", Name: "Change password", Group: "Account", Component: authview.ChangePassword},
		{Slug: "reauthenticate", Name: "Reauthenticate", Group: "Account", Component: func() templ.Component {
			return authview.Reauthenticate(true, true, "showcase-google-client", true, "fixture-challenge", false)
		}},
		{Slug: "reauthentication-complete", Name: "Reauthentication complete", Group: "Account", Component: func() templ.Component { return authview.ReauthenticationComplete(false) }},
		{Slug: "error", Name: "Error page", Group: "System", Component: func() templ.Component {
			return templates.ErrorPage(http.StatusNotFound, "The requested page could not be found.")
		}},
	}
	return pages
}

var fixtureSessionID = uuid.MustParse("22222222-2222-2222-2222-222222222222")

func fixtureAccount() templ.Component {
	return userview.Account(userview.AccountConfig{
		Username: "alex", Email: "alex@example.com", OperatorAccess: true, GoogleClientID: "showcase-google-client", CurrentSessionID: fixtureSessionID,
		Sessions: []viewmodel.Session{
			{ID: fixtureSessionID, IsCurrent: true, Title: "Current session", Subtitle: "Safari on macOS · Berlin", DeviceKind: viewmodel.DeviceDesktop},
			{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Title: "Chrome on iPhone", Subtitle: "Created Sep 20, 2026", DeviceKind: viewmodel.DevicePhone},
		},
		AuthProviders: []viewmodel.AuthProvider{
			{ID: "password", Kind: viewmodel.AuthProviderPassword, Title: "Password", Subtitle: "Use your email and password to sign in.", Linked: true, Primary: true, ActionLabel: "Change password"},
			{ID: "google", Kind: viewmodel.AuthProviderGoogle, Title: "Google", Subtitle: "alex@example.com", Linked: true, ActionLabel: "Unlink"},
		},
		Passkeys: []viewmodel.Passkey{{ID: uuid.MustParse("44444444-4444-4444-4444-444444444444"), Name: "MacBook Touch ID", CreatedAt: "Sep 12, 2026", LastUsedAt: "Sep 27, 2026", CanDelete: true}},
	})
}

var galleryTemplate = template.Must(template.New("showcase").Funcs(template.FuncMap{
	"groupChanged": func(pages []Page, index int) bool { return index == 0 || pages[index-1].Group != pages[index].Group },
}).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Authara UI showcase</title>
  <style>
    *{box-sizing:border-box}html{background:#09090b;color:#fafafa;font-family:Inter,ui-sans-serif,system-ui,sans-serif}body{margin:0;padding:32px}header{max-width:72rem;margin:0 auto 32px}h1{margin:0;font-size:clamp(2rem,5vw,4rem)}header p{max-width:44rem;color:#a1a1aa;line-height:1.6}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,420px),1fr));gap:24px;max-width:96rem;margin:auto}.group{grid-column:1/-1;margin:28px 0 0;color:#a1a1aa;font-size:.75rem;letter-spacing:.14em;text-transform:uppercase}.card{overflow:hidden;border:1px solid #27272a;border-radius:16px;background:#18181b}.label{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:12px 16px}.label a{color:#fafafa;text-decoration:none;font-weight:650}.label code{color:#a1a1aa;font-size:.75rem}.frame{height:720px;background:white}.frame iframe{display:block;width:100%;height:100%;border:0}
  </style>
</head>
<body>
  <header><h1>Authara UI showcase</h1><p>Real Core templates rendered with deterministic fixture data. This route exists only when <code>APP_ENV=dev</code>.</p></header>
  <main class="grid">
  {{range $i, $page := .}}
    {{if groupChanged $ $i}}<h2 class="group">{{$page.Group}}</h2>{{end}}
    <section class="card"><div class="label"><a href="/auth/showcase/pages/{{$page.Slug}}" target="_blank">{{$page.Name}}</a><code>{{$page.Slug}}</code></div><div class="frame"><iframe loading="lazy" sandbox title="{{$page.Name}}" src="/auth/showcase/pages/{{$page.Slug}}"></iframe></div></section>
  {{end}}
  </main>
</body>
</html>`))
