package ui

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	authhandler "github.com/authara-org/authara/internal/http/handlers/auth"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/organization"
	authsession "github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func (h *UIHandler) InvitationAcceptPage(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid invitation.")
		return
	}

	now := time.Now().UTC()
	preview, err := h.Organizations.InvitationByToken(r.Context(), token)
	if err != nil {
		h.renderInvitationError(w, r, err)
		return
	}

	statusMessage := invitationStatusMessage(preview.Invitation.Status(now))

	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		errorMessage := statusMessage
		title := "You have been invited to join " + preview.Organization.Name
		description := "Sign in or create an account with " + preview.Invitation.Email + " to accept this invitation."
		showSignupForm := false
		actions := []authview.InvitationAction{
			{Label: "Log in with invited email", Href: invitationAuthURL("/auth/invitations/login", token), Primary: true},
			{Label: "Create account", Href: invitationAuthURL("/auth/invitations/signup", token)},
		}
		if h.Organizations.Mode() == organization.OrgModeSingle {
			title = "Create an account to join " + preview.Organization.Name
			description = "This invitation can only be accepted by creating an account for " + preview.Invitation.Email + "."
			actions = nil
			showSignupForm = true
			if statusMessage == "" {
				exists, err := h.Auth.UserExistsByEmail(r.Context(), preview.Invitation.Email)
				if err != nil {
					h.renderInternalError(w, r)
					return
				}
				if exists {
					title = "Invitation requires a new account"
					description = "This invitation was sent to an email that already has an account."
					errorMessage = "This email already belongs to an organization."
					showSignupForm = false
				}
			}
		}
		if statusMessage != "" {
			title = "Invitation unavailable"
			description = "This invitation cannot be accepted."
			actions = nil
			showSignupForm = false
		}
		h.renderInvitationPage(w, r, invitationPage{
			Preview:        preview,
			Token:          token,
			Title:          title,
			Description:    description,
			ErrorMessage:   errorMessage,
			Actions:        actions,
			ShowSignupForm: showSignupForm,
		})
		return
	}
	user, err := h.Auth.GetUser(r.Context(), userID)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}

	page, err := h.invitationPageForUser(r.Context(), preview, token, user)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}
	if statusMessage != "" {
		page.Title = "Invitation unavailable"
		page.Description = "This invitation cannot be accepted."
		page.ErrorMessage = statusMessage
		page.AcceptLabel = ""
		page.Actions = nil
	}
	h.renderInvitationPage(w, r, page)
}

type invitationPage struct {
	Preview        organization.InvitationPreview
	Token          string
	CurrentEmail   string
	Title          string
	Description    string
	ErrorMessage   string
	AcceptLabel    string
	Actions        []authview.InvitationAction
	ShowSignupForm bool
}

func (h *UIHandler) renderInvitationPage(w http.ResponseWriter, r *http.Request, page invitationPage) {
	if page.ShowSignupForm {
		r = r.WithContext(httpctx.WithReturnTo(
			r.Context(),
			invitationAuthURL("/auth/invitations/signup", page.Token),
		))
	}
	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.InvitationAccept(
			page.Preview.Organization.Name,
			page.Preview.Invitation.Email,
			page.CurrentEmail,
			string(page.Preview.Invitation.Role),
			page.Preview.Invitation.ExpiresAt.Local().Format(time.RFC1123),
			page.Token,
			page.Title,
			page.Description,
			page.ErrorMessage,
			page.AcceptLabel,
			page.Actions,
			page.ShowSignupForm,
			h.OAuthProviders.Providers,
		),
	)
}

func (h *UIHandler) invitationPageForUser(
	ctx context.Context,
	preview organization.InvitationPreview,
	rawToken string,
	user domain.User,
) (invitationPage, error) {
	emailMatches := normalizeEmailForDisplay(user.Email) == normalizeEmailForDisplay(preview.Invitation.Email)
	memberOfInvitedOrg, _, err := h.invitationMembershipState(ctx, user.ID, preview.Organization.ID)
	if err != nil {
		return invitationPage{}, err
	}

	page := invitationPage{
		Preview:      preview,
		Token:        rawToken,
		CurrentEmail: user.Email,
		Title:        "You have been invited to join " + preview.Organization.Name,
		Description:  "Choose which Authara account should accept this invitation.",
	}

	switch h.Organizations.Mode() {
	case organization.OrgModeSingle:
		switch {
		case memberOfInvitedOrg:
			page.Title = "You are already a member of " + preview.Organization.Name
			page.Description = "Continue to the application."
			page.Actions = []authview.InvitationAction{{Label: "Continue", Href: "/", Primary: true}}
		case emailMatches:
			page.Title = "Invitation requires a new account"
			page.Description = "This invitation was sent to an email that already has an account."
			page.ErrorMessage = "In single-organization mode, existing accounts cannot accept organization invitations."
		default:
			page.Title = "Create an account to join " + preview.Organization.Name
			page.Description = "This invitation can only be accepted by creating an account for " + preview.Invitation.Email + "."
			page.ShowSignupForm = true
		}
	case organization.OrgModeMulti:
		if emailMatches {
			page.Description = "You are currently signed in as " + user.Email + "."
			page.AcceptLabel = "Join " + preview.Organization.Name + " as " + user.Email
			page.Actions = []authview.InvitationAction{{Label: "Use another account", Href: invitationAuthURL("/auth/invitations/login", rawToken)}}
		} else {
			page.Description = "This invitation was sent to " + preview.Invitation.Email + ". You are signed in as " + user.Email + "."
			page.Actions = []authview.InvitationAction{
				{Label: "Log in with invited email", Href: invitationAuthURL("/auth/invitations/login", rawToken), Primary: true},
				{Label: "Create account", Href: invitationAuthURL("/auth/invitations/signup", rawToken)},
			}
		}
	default:
		page.Title = "Invitation unavailable"
		page.Description = "Organization invitations are not enabled."
		page.ErrorMessage = "Organization invitations are not enabled."
	}

	return page, nil
}

func (h *UIHandler) invitationMembershipState(ctx context.Context, userID uuid.UUID, organizationID uuid.UUID) (bool, bool, error) {
	_, err := h.Organizations.RequireMembership(ctx, userID, organizationID)
	switch {
	case err == nil:
		return true, true, nil
	case errors.Is(err, store.ErrOrganizationMembershipNotFound):
	default:
		return false, false, err
	}

	_, _, err = h.Organizations.DefaultOrganizationForUser(ctx, userID)
	switch {
	case err == nil:
		return false, true, nil
	case errors.Is(err, store.ErrOrganizationNotFound):
		return false, false, nil
	default:
		return false, false, err
	}
}

func (h *UIHandler) InvitationAcceptPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid invitation.")
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid invitation.")
		return
	}

	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		h.renderUnauthorized(w, r)
		return
	}
	if !h.Organizations.Mode().AllowsOrgSwitching() {
		h.renderRequestError(w, r, http.StatusForbidden, "This invitation requires creating a new account.")
		return
	}

	now := time.Now().UTC()
	result, err := h.Organizations.AcceptInvitation(r.Context(), organization.AcceptInvitationInput{
		RawToken: token,
		UserID:   userID,
		Now:      now,
	})
	if err != nil {
		h.renderInvitationError(w, r, err)
		return
	}

	sessionID, ok := httpctx.SessionID(r.Context())
	if !ok {
		h.renderInternalError(w, r)
		return
	}

	accessToken, refreshToken, err := h.Session.SwitchSessionOrganization(
		r.Context(),
		userID,
		sessionID,
		result.Organization.ID,
		redirect.AudienceForPath("/"),
		now,
	)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}
	authsession.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	authsession.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	redirect.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *UIHandler) InvitationSignupPage(w http.ResponseWriter, r *http.Request) {
	preview, token, ok := h.invitationPreviewFromQuery(w, r)
	if !ok {
		return
	}

	r = r.WithContext(httpctx.WithReturnTo(r.Context(), invitationAuthURL("/auth/invitations/signup", token)))
	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.InvitationSignup(preview.Organization.Name, preview.Invitation.Email, token, h.OAuthProviders.Providers),
	)
}

func (h *UIHandler) InvitationSignupPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid invitation.")
		return
	}
	preview, token, ok := h.invitationPreviewFromForm(w, r)
	if !ok {
		return
	}

	password := r.FormValue("password")
	if !authhandler.IsValidPassword(password) {
		h.renderInvitationSignupError(w, r, http.StatusUnprocessableEntity, "Please provide a valid password.", preview, token)
		return
	}

	exists, err := h.Auth.UserExistsByEmail(r.Context(), preview.Invitation.Email)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}
	if exists {
		msg := "An account already exists for this invitation email. Use invite login instead."
		if h.Organizations.Mode() == organization.OrgModeSingle {
			msg = "An account already exists for this invitation email. In single-organization mode, existing accounts cannot accept invitations."
		}
		h.renderInvitationSignupError(
			w,
			r,
			http.StatusUnprocessableEntity,
			msg,
			preview,
			token,
		)
		return
	}

	ip := httputil.ClientIP(r)
	allowed, err := h.Limiter.AllowSignupAttempt(r.Context(), ip, preview.Invitation.Email)
	if err != nil || !allowed {
		h.renderInvitationSignupError(w, r, http.StatusTooManyRequests, "Too many attempts. Please try again later.", preview, token)
		return
	}

	passwordHash, err := auth.Hash(password)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}

	if h.Features.ChallengeEnabled {
		invitationID := preview.Invitation.ID
		challengeID, err := h.Challenge.CreateSignupChallenge(r.Context(), challenge.CreateSignupChallengeInput{
			Email:        preview.Invitation.Email,
			PasswordHash: passwordHash,
			InvitationID: &invitationID,
		}, time.Now().UTC())
		if err != nil {
			h.renderInvitationSignupError(w, r, http.StatusUnprocessableEntity, "Could not start signup verification. Please try again.", preview, token)
			return
		}

		_ = h.renderVerifyChallengeRedirect(
			w,
			r,
			VerifyChallengeActionSignup,
			challengeID.String(),
			"/",
		)
		return
	}

	user, err := h.Auth.Signup(r.Context(), auth.SignupInput{
		Provider:        domain.ProviderPassword,
		Email:           preview.Invitation.Email,
		PasswordHash:    passwordHash,
		InvitationToken: token,
	})
	if err != nil {
		h.renderInvitationSignupError(w, r, http.StatusUnprocessableEntity, "Could not create account for this invitation.", preview, token)
		return
	}

	h.finishInvitationSession(w, r, user, token, time.Now())
}

func (h *UIHandler) InvitationLoginPage(w http.ResponseWriter, r *http.Request) {
	preview, token, ok := h.invitationPreviewFromQuery(w, r)
	if !ok {
		return
	}
	if !h.Organizations.Mode().AllowsOrgSwitching() {
		h.renderRequestError(w, r, http.StatusForbidden, "This invitation requires creating a new account.")
		return
	}

	r = r.WithContext(httpctx.WithReturnTo(r.Context(), invitationAuthURL("/auth/invitations/login", token)))
	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.InvitationLogin(preview.Organization.Name, preview.Invitation.Email, token, h.OAuthProviders.Providers),
	)
}

func (h *UIHandler) InvitationLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid invitation.")
		return
	}
	preview, token, ok := h.invitationPreviewFromForm(w, r)
	if !ok {
		return
	}
	if !h.Organizations.Mode().AllowsOrgSwitching() {
		h.renderRequestError(w, r, http.StatusForbidden, "This invitation requires creating a new account.")
		return
	}

	password := r.FormValue("password")
	if password == "" {
		h.renderInvitationLoginError(w, r, http.StatusBadRequest, "Password required.", preview, token)
		return
	}

	ip := httputil.ClientIP(r)
	allowed, err := h.Limiter.AllowLoginAttempt(r.Context(), ip, preview.Invitation.Email)
	if err != nil || !allowed {
		h.renderInvitationLoginError(w, r, http.StatusTooManyRequests, "Too many attempts. Please try again later.", preview, token)
		return
	}

	user, err := h.Auth.Login(r.Context(), auth.LoginInput{
		Provider: domain.ProviderPassword,
		Email:    preview.Invitation.Email,
		Password: password,
	})
	if err != nil {
		h.renderInvitationLoginError(w, r, http.StatusUnprocessableEntity, "Invalid email or password.", preview, token)
		return
	}

	if _, err := h.Organizations.AcceptInvitation(r.Context(), organization.AcceptInvitationInput{
		RawToken: token,
		UserID:   user.ID,
		Now:      time.Now().UTC(),
	}); err != nil {
		h.renderInvitationError(w, r, err)
		return
	}

	h.finishInvitationSession(w, r, user, token, time.Now())
}

func (h *UIHandler) invitationPreviewFromQuery(w http.ResponseWriter, r *http.Request) (organization.InvitationPreview, string, bool) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid invitation.")
		return organization.InvitationPreview{}, "", false
	}
	return h.invitationPreview(w, r, token)
}

func (h *UIHandler) invitationPreviewFromForm(w http.ResponseWriter, r *http.Request) (organization.InvitationPreview, string, bool) {
	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid invitation.")
		return organization.InvitationPreview{}, "", false
	}
	return h.invitationPreview(w, r, token)
}

func (h *UIHandler) invitationPreview(w http.ResponseWriter, r *http.Request, token string) (organization.InvitationPreview, string, bool) {
	preview, err := h.Organizations.InvitationByToken(r.Context(), token)
	if err != nil {
		h.renderInvitationError(w, r, err)
		return organization.InvitationPreview{}, "", false
	}
	if msg := invitationStatusMessage(preview.Invitation.Status(time.Now().UTC())); msg != "" {
		h.renderRequestError(w, r, http.StatusGone, msg)
		return organization.InvitationPreview{}, "", false
	}
	return preview, token, true
}

func (h *UIHandler) finishInvitationSession(w http.ResponseWriter, r *http.Request, user domain.User, token string, now time.Time) {
	accessToken, refreshToken, err := h.Session.CreateSession(r.Context(), user.ID, redirect.AudienceForPath("/"), r.UserAgent(), now)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}
	accessToken, refreshToken, err = h.switchSessionToInvitationOrganization(r.Context(), user.ID, accessToken, token, redirect.AudienceForPath("/"), now)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}

	if currentRefresh, ok := authsession.ReadRefreshToken(r); ok {
		_ = h.Session.Logout(r.Context(), currentRefresh)
	}
	authsession.ClearSessionCookies(w)
	authsession.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	authsession.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))
	redirect.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *UIHandler) finishInvitationSessionByID(w http.ResponseWriter, r *http.Request, user domain.User, invitationID uuid.UUID, now time.Time) {
	accessToken, refreshToken, err := h.Session.CreateSession(r.Context(), user.ID, redirect.AudienceForPath("/"), r.UserAgent(), now)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}
	accessToken, refreshToken, err = h.switchSessionToInvitationOrganizationByID(r.Context(), user.ID, accessToken, invitationID, redirect.AudienceForPath("/"), now)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}

	if currentRefresh, ok := authsession.ReadRefreshToken(r); ok {
		_ = h.Session.Logout(r.Context(), currentRefresh)
	}
	authsession.ClearSessionCookies(w)
	authsession.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	authsession.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))
	redirect.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *UIHandler) finishInvitationOAuth(
	w http.ResponseWriter,
	r *http.Request,
	path string,
	token string,
	returnTo string,
	email string,
	oauthID string,
) {
	preview, err := h.Organizations.InvitationByToken(r.Context(), token)
	if err != nil {
		h.redirectInvitationOAuthFailure(w, returnTo)
		return
	}
	if normalizeEmailForDisplay(email) != normalizeEmailForDisplay(preview.Invitation.Email) {
		h.redirectInvitationOAuthFailure(w, returnTo)
		return
	}

	exists, err := h.Auth.UserExistsByEmail(r.Context(), preview.Invitation.Email)
	if err != nil {
		h.redirectInvitationOAuthFailure(w, returnTo)
		return
	}
	if path == "/auth/invitations/signup" && exists {
		h.redirectInvitationOAuthFailure(w, returnTo)
		return
	}
	if path == "/auth/invitations/login" {
		if !h.Organizations.Mode().AllowsOrgSwitching() || !exists {
			h.redirectInvitationOAuthFailure(w, returnTo)
			return
		}
	}

	input := auth.LoginInput{
		Provider: domain.ProviderGoogle,
		Email:    preview.Invitation.Email,
		OAuthID:  oauthID,
	}
	if path == "/auth/invitations/signup" {
		input.InvitationToken = token
	}

	user, err := h.Auth.Login(r.Context(), input)
	if err != nil {
		h.redirectInvitationOAuthFailure(w, returnTo)
		return
	}

	if path == "/auth/invitations/login" {
		if _, err := h.Organizations.AcceptInvitation(r.Context(), organization.AcceptInvitationInput{
			RawToken: token,
			UserID:   user.ID,
			Now:      time.Now().UTC(),
		}); err != nil {
			h.redirectInvitationOAuthFailure(w, returnTo)
			return
		}
	}

	h.finishInvitationSession(w, r, user, token, time.Now())
}

func (h *UIHandler) redirectInvitationOAuthFailure(w http.ResponseWriter, returnTo string) {
	w.Header().Set("X-Authara-Redirect", returnTo)
	w.WriteHeader(http.StatusOK)
}

func (h *UIHandler) renderInvitationSignupError(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	msg string,
	preview organization.InvitationPreview,
	token string,
) {
	h.renderFormError(w, r, status, msg, authview.InvitationSignupForm(preview.Invitation.Email, token))
}

func (h *UIHandler) renderInvitationLoginError(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	msg string,
	preview organization.InvitationPreview,
	token string,
) {
	h.renderFormError(w, r, status, msg, authview.InvitationLoginForm(preview.Invitation.Email, token))
}

func invitationStatusMessage(status domain.OrganizationInvitationStatus) string {
	switch status {
	case domain.OrganizationInvitationStatusAccepted:
		return "This invitation has already been accepted."
	case domain.OrganizationInvitationStatusRevoked:
		return "This invitation has been revoked."
	case domain.OrganizationInvitationStatusExpired:
		return "This invitation has expired."
	default:
		return ""
	}
}

func (h *UIHandler) renderInvitationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrOrganizationInvitationNotFound),
		errors.Is(err, organization.ErrInvalidOrganizationInvitationToken):
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid invitation.")
	case errors.Is(err, organization.ErrOrganizationInvitationExpired):
		h.renderRequestError(w, r, http.StatusGone, "This invitation has expired.")
	case errors.Is(err, organization.ErrOrganizationInvitationRevoked):
		h.renderRequestError(w, r, http.StatusGone, "This invitation has been revoked.")
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted):
		h.renderRequestError(w, r, http.StatusConflict, "This invitation has already been accepted.")
	case errors.Is(err, organization.ErrOrganizationInviteEmailMismatch):
		h.renderRequestError(w, r, http.StatusForbidden, "This invitation is for a different account.")
	case errors.Is(err, organization.ErrOrganizationInviteForbidden):
		h.renderRequestError(w, r, http.StatusForbidden, "Organization invitations are disabled.")
	case errors.Is(err, organization.ErrOrganizationSingleMembershipConflict):
		h.renderRequestError(w, r, http.StatusConflict, "This account already belongs to another organization.")
	default:
		h.renderInternalError(w, r)
	}
}

func normalizeEmailForDisplay(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func invitationAuthReturnTo(returnTo string) (path string, token string, ok bool) {
	u, err := url.Parse(returnTo)
	if err != nil || u.IsAbs() || (u.Path != "/auth/invitations/login" && u.Path != "/auth/invitations/signup") {
		return "", "", false
	}
	token = strings.TrimSpace(u.Query().Get("token"))
	if token == "" {
		return "", "", false
	}
	return u.Path, token, true
}

func invitationAuthURL(path string, token string) string {
	u := url.URL{Path: path}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String()
}

func isInvitationAuthPath(path string) bool {
	return path == "/auth/invitations/accept" ||
		path == "/auth/invitations/login" ||
		path == "/auth/invitations/signup"
}

func (h *UIHandler) invitationAcceptedByUser(ctx context.Context, token string, userID uuid.UUID) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}
	preview, err := h.Organizations.InvitationByToken(ctx, token)
	return err == nil && preview.Invitation.AcceptedByUserID != nil && *preview.Invitation.AcceptedByUserID == userID
}

func (h *UIHandler) switchSessionToInvitationOrganization(
	ctx context.Context,
	userID uuid.UUID,
	accessToken string,
	invitationToken string,
	audience token.Audience,
	now time.Time,
) (string, string, error) {
	identity, err := h.Session.ValidateAccessToken(accessToken, audience, now)
	if err != nil {
		return "", "", err
	}
	preview, err := h.Organizations.InvitationByToken(ctx, invitationToken)
	if err != nil {
		return "", "", err
	}
	return h.Session.SwitchSessionOrganization(ctx, userID, identity.SessionID, preview.Organization.ID, audience, now)
}

func (h *UIHandler) switchSessionToInvitationOrganizationByID(
	ctx context.Context,
	userID uuid.UUID,
	accessToken string,
	invitationID uuid.UUID,
	audience token.Audience,
	now time.Time,
) (string, string, error) {
	identity, err := h.Session.ValidateAccessToken(accessToken, audience, now)
	if err != nil {
		return "", "", err
	}
	preview, err := h.Organizations.InvitationByID(ctx, invitationID)
	if err != nil {
		return "", "", err
	}
	return h.Session.SwitchSessionOrganization(ctx, userID, identity.SessionID, preview.Organization.ID, audience, now)
}
