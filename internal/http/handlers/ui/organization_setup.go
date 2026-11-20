package ui

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func organizationSetupReturnTo(credentials session.Credentials, returnTo string) string {
	if credentials.ActiveOrganizationID != uuid.Nil || strings.HasPrefix(returnTo, "/auth/invitations/") {
		return returnTo
	}
	return redirect.WithReturnTo("/auth/organizations/setup", returnTo)
}

func (h *UIHandler) OrganizationSetupPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := httpctx.OrganizationID(r.Context()); ok {
		redirect.Redirect(w, r, httpctx.ReturnToOrManualDefault(r.Context(), "/"), http.StatusSeeOther)
		return
	}
	_ = h.Render(w, r, http.StatusOK, authview.OrganizationSetup())
}

func (h *UIHandler) OrganizationSetupPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderFormError(w, r, http.StatusBadRequest, "Invalid form.", authview.OrganizationSetupForms())
		return
	}
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		h.renderUnauthorized(w, r)
		return
	}
	sessionID, ok := httpctx.SessionID(r.Context())
	if !ok {
		h.renderUnauthorized(w, r)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	org, _, err := h.Organizations.CreateOrganizationForOrglessUser(r.Context(), organization.CreateOrganizationInput{
		Name:            name,
		CreatedByUserID: userID,
	})
	switch {
	case errors.Is(err, store.ErrInvalidOrganizationName):
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Enter a valid organization name.", authview.OrganizationSetupForms())
		return
	case errors.Is(err, organization.ErrOrganizationSingleMembershipConflict):
		h.renderFormError(w, r, http.StatusConflict, "This account already belongs to an organization. Sign in again to continue.", authview.OrganizationSetupForms())
		return
	case err != nil:
		h.renderInternalError(w, r)
		return
	}
	now := time.Now().UTC()
	accessToken, refreshToken, err := h.Session.SwitchSessionOrganization(r.Context(), userID, sessionID, org.ID, token.AudienceApp, now)
	if err != nil {
		h.renderInternalError(w, r)
		return
	}
	cookiePolicy := h.sessionCookiePolicy()
	session.SetAccessToken(w, accessToken, int(cookiePolicy.AccessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(cookiePolicy.RefreshTokenTTL.Seconds()))
	redirect.Redirect(w, r, httpctx.ReturnToOrManualDefault(r.Context(), "/"), http.StatusSeeOther)
}
