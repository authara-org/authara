package ui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/http/viewmodel"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/session"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type passkeyFinishRequest struct {
	AuthenticationChallengeID string          `json:"authentication_challenge_id"`
	ChallengeID               string          `json:"challenge_id"`
	Credential                json.RawMessage `json:"credential"`
	Name                      string          `json:"name"`
	PlatformHint              string          `json:"platform_hint"`
	ReturnTo                  string          `json:"return_to"`
}

const passkeyResponseLinkedProvidersSection = "linked-providers-section"

func (h *UIHandler) PasskeySetupPage(w http.ResponseWriter, r *http.Request) {
	returnTo := httpctx.ReturnToOrDefault(r.Context())

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.PasskeySetup(returnTo),
	)
}

func (h *UIHandler) PasskeyRegisterOptionsPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Unauthorized.")
		return
	}
	if h.Passkeys == nil {
		response.ErrorJSON(w, http.StatusServiceUnavailable, response.CodeInternalError, "Passkeys are not available.")
		return
	}

	optionsJSON, _, err := h.Passkeys.BeginRegistration(ctx, userID)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("begin passkey registration failed", "err", err)
		}
		response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Could not start passkey setup.")
		return
	}

	response.RawJSON(w, http.StatusOK, optionsJSON)
}

func (h *UIHandler) PasskeyRegisterFinishPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Unauthorized.")
		return
	}
	if h.Passkeys == nil {
		response.ErrorJSON(w, http.StatusServiceUnavailable, response.CodeInternalError, "Passkeys are not available.")
		return
	}

	in, err := decodePasskeyFinishRequest(r)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid passkey response.")
		return
	}

	challengeID, err := uuid.Parse(in.ChallengeID)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid passkey challenge.")
		return
	}

	if err := h.Passkeys.FinishRegistration(ctx, userID, challengeID, in.Credential, passkey.RegistrationMetadata{
		Name:         in.Name,
		UserAgent:    r.UserAgent(),
		PlatformHint: in.PlatformHint,
	}); err != nil {
		status := http.StatusUnprocessableEntity
		msg := "Could not add passkey."
		switch {
		case errors.Is(err, passkey.ErrPasskeyAlreadyExists):
			msg = "This passkey is already linked to an account."
		case errors.Is(err, passkey.ErrPasskeyRegistrationInvalid):
			msg = "Passkey setup could not be verified."
		default:
			if h.Logger != nil {
				h.Logger.Error("finish passkey registration failed", "err", err)
			}
			status = http.StatusInternalServerError
			msg = "Something went wrong."
		}
		response.ErrorJSON(w, status, response.CodeInvalidRequest, msg)
		return
	}

	if r.Header.Get("X-Authara-Response") == passkeyResponseLinkedProvidersSection {
		section, err := h.linkedProvidersSection(ctx)
		if err != nil {
			if h.Logger != nil {
				h.Logger.Error("load sign-in methods after passkey registration failed", "err", err)
			}
			response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Could not load sign-in methods.")
			return
		}

		_ = h.Render(w, r, http.StatusOK, section)
		return
	}

	returnTo := normalizedReturnTo(in.ReturnTo, httpctx.ReturnToOrManualDefault(ctx, "/auth/account"))
	response.JSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"return_to": returnTo,
	})
}

func (h *UIHandler) PasskeyAuthenticateOptionsPost(w http.ResponseWriter, r *http.Request) {
	if h.Passkeys == nil {
		response.ErrorJSON(w, http.StatusServiceUnavailable, response.CodeInternalError, "Passkeys are not available.")
		return
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginAttempt(r.Context(), httputil.ClientIP(r))
		if err != nil || !allowed {
			response.ErrorJSON(w, http.StatusTooManyRequests, response.CodeRateLimited, "Too many attempts. Please try again later.")
			return
		}
	}

	optionsJSON, _, err := h.Passkeys.BeginLogin(r.Context())
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("begin passkey login failed", "err", err)
		}
		response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Could not start passkey login.")
		return
	}

	response.RawJSON(w, http.StatusOK, optionsJSON)
}

func (h *UIHandler) PasskeyAuthenticateFinishPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.Passkeys == nil {
		response.ErrorJSON(w, http.StatusServiceUnavailable, response.CodeInternalError, "Passkeys are not available.")
		return
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginFinishAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			response.ErrorJSON(w, http.StatusTooManyRequests, response.CodeRateLimited, "Too many attempts. Please try again later.")
			return
		}
	}

	in, err := decodePasskeyFinishRequest(r)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid passkey response.")
		return
	}

	challengeID, err := uuid.Parse(in.ChallengeID)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid passkey challenge.")
		return
	}

	now := time.Now().UTC()
	result, err := h.Passkeys.FinishLogin(ctx, challengeID, in.Credential, now)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("passkey login failed", "err", err)
		}
		response.ErrorJSON(w, http.StatusUnprocessableEntity, response.CodeInvalidRequest, "Passkey sign-in failed.")
		return
	}
	if !result.Decision.AllowSession {
		response.ErrorJSON(w, http.StatusUnprocessableEntity, response.CodeInvalidRequest, "Passkey sign-in failed.")
		return
	}

	returnTo := normalizedReturnTo(in.ReturnTo, httpctx.ReturnToOrDefault(ctx))
	audience := redirect.AudienceForPath(returnTo)
	accessToken, refreshToken, err := h.Session.CreatePasskeySession(ctx, result.User.ID, result.PasskeyID, audience, r.UserAgent(), now, httputil.ClientIPString(r))
	if err != nil {
		if errors.Is(err, session.ErrAuthenticationMethodUnavailable) {
			response.ErrorJSON(w, http.StatusUnprocessableEntity, response.CodeInvalidRequest, "Passkey sign-in failed.")
			return
		}
		response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Could not create session.")
		return
	}

	cookiePolicy := h.sessionCookiePolicy()
	session.SetAccessToken(w, accessToken, int(cookiePolicy.AccessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(cookiePolicy.RefreshTokenTTL.Seconds()))

	response.JSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"return_to": returnTo,
	})
}

func (h *UIHandler) ReauthenticatePasskeyOptionsPost(w http.ResponseWriter, r *http.Request) {
	userID, userOK := httpctx.UserID(r.Context())
	sessionID, sessionOK := httpctx.SessionID(r.Context())
	if !userOK || !sessionOK {
		response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Unauthorized.")
		return
	}
	if h.Passkeys == nil {
		response.ErrorJSON(w, http.StatusServiceUnavailable, response.CodeInternalError, "Passkeys are not available.")
		return
	}
	authenticationChallengeID, err := decodeAuthenticationChallengeReference(r)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Authentication challenge required.")
		return
	}
	if err := h.Session.ValidateAuthenticationChallenge(r.Context(), userID, sessionID, authenticationChallengeID, time.Now().UTC()); err != nil {
		writeAuthenticationChallengeJSONError(w, err)
		return
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginAttempt(r.Context(), httputil.ClientIP(r))
		if err != nil || !allowed {
			response.ErrorJSON(w, http.StatusTooManyRequests, response.CodeRateLimited, "Too many attempts. Please try again later.")
			return
		}
	}
	optionsJSON, _, err := h.Passkeys.BeginReauthentication(r.Context(), userID, sessionID)
	if err != nil {
		response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Could not start passkey authentication.")
		return
	}
	response.RawJSON(w, http.StatusOK, optionsJSON)
}

func (h *UIHandler) ReauthenticatePasskeyFinishPost(w http.ResponseWriter, r *http.Request) {
	userID, userOK := httpctx.UserID(r.Context())
	sessionID, sessionOK := httpctx.SessionID(r.Context())
	if !userOK || !sessionOK {
		response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Unauthorized.")
		return
	}
	if h.Passkeys == nil {
		response.ErrorJSON(w, http.StatusServiceUnavailable, response.CodeInternalError, "Passkeys are not available.")
		return
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginFinishAttempt(r.Context(), httputil.ClientIP(r))
		if err != nil || !allowed {
			response.ErrorJSON(w, http.StatusTooManyRequests, response.CodeRateLimited, "Too many attempts. Please try again later.")
			return
		}
	}
	in, err := decodePasskeyFinishRequest(r)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid passkey response.")
		return
	}
	challengeID, err := uuid.Parse(in.ChallengeID)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid passkey challenge.")
		return
	}
	authenticationChallengeID, err := uuid.Parse(in.AuthenticationChallengeID)
	if err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Authentication challenge required.")
		return
	}
	now := time.Now().UTC()
	if err := h.Session.ValidateAuthenticationChallenge(r.Context(), userID, sessionID, authenticationChallengeID, now); err != nil {
		writeAuthenticationChallengeJSONError(w, err)
		return
	}
	decision, err := h.Passkeys.FinishReauthentication(r.Context(), userID, sessionID, challengeID, in.Credential, now)
	if err != nil {
		response.ErrorJSON(w, http.StatusUnprocessableEntity, response.CodeInvalidRequest, "Passkey authentication failed.")
		return
	}
	if !decision.AllowSession {
		response.ErrorJSON(w, http.StatusUnprocessableEntity, response.CodeInvalidRequest, "Passkey authentication failed.")
		return
	}
	if err := h.Session.CompleteAuthenticationChallenge(r.Context(), userID, sessionID, authenticationChallengeID, domain.AuthenticationMethodPasskey, now); err != nil {
		writeAuthenticationChallengeJSONError(w, err)
		return
	}
	returnTo := normalizedReturnTo(in.ReturnTo, httpctx.ReturnToOrManualDefault(r.Context(), "/auth/account"))
	response.JSON(w, http.StatusOK, map[string]any{"ok": true, "return_to": returnTo})
}

func (h *UIHandler) PasskeyDeletePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		h.renderUnauthorized(w, r)
		return
	}
	if h.Passkeys == nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusServiceUnavailable, toast.ToastMessage(toast.Error, "Passkeys are not available."))
		return
	}

	passkeyID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusBadRequest, toast.ToastMessage(toast.Error, "Invalid passkey."))
		return
	}

	if err := h.Passkeys.DeletePasskey(ctx, userID, passkeyID); err != nil {
		htmx.ReSwap(w, "none")
		status := http.StatusInternalServerError
		msg := "Could not remove passkey."
		switch {
		case errors.Is(err, passkey.ErrCannotRemoveLastAuthMethod):
			status = http.StatusUnprocessableEntity
			msg = "You need at least one sign-in method."
		case errors.Is(err, passkey.ErrPasskeyNotFound):
			status = http.StatusNotFound
			msg = "Passkey not found."
		default:
			if h.Logger != nil {
				h.Logger.Error("delete passkey failed", "err", err)
			}
		}
		_ = h.Render(w, r, status, toast.ToastMessage(toast.Error, msg))
		return
	}

	section, err := h.linkedProvidersSection(ctx)
	if err != nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusInternalServerError, toast.ToastMessage(toast.Error, "Could not load sign-in methods."))
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		templ.Join(
			toast.ToastMessage(toast.Success, "Passkey removed."),
			section,
		),
	)
}

func (h *UIHandler) linkedProvidersSection(ctx context.Context) (templ.Component, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return nil, errors.New("missing user id")
	}

	providers, err := h.Auth.ListUserAuthProviders(ctx, userID)
	if err != nil {
		return nil, err
	}

	var passkeys []domain.Passkey
	if h.Passkeys != nil {
		page, pageErr := h.Passkeys.ListUserPasskeysPage(ctx, userID, passkey.ListOptions{})
		err = pageErr
		if err != nil {
			return nil, err
		}
		passkeys = page.Items
	}

	total := len(providers) + len(passkeys)
	return userview.LinkedProvidersSection(
		viewmodel.AuthProvidersFromDomain(providers, h.OAuthProviders.Providers),
		viewmodel.PasskeysFromDomain(passkeys, total),
		h.Google.ClientID,
	), nil
}

func decodePasskeyFinishRequest(r *http.Request) (passkeyFinishRequest, error) {
	defer r.Body.Close()

	body, err := io.ReadAll(io.LimitReader(r.Body, 128*1024))
	if err != nil {
		return passkeyFinishRequest{}, err
	}

	var in passkeyFinishRequest
	if err := json.Unmarshal(body, &in); err != nil {
		return passkeyFinishRequest{}, err
	}
	if in.ChallengeID == "" || len(in.Credential) == 0 {
		return passkeyFinishRequest{}, errors.New("missing challenge or credential")
	}

	return in, nil
}

func decodeAuthenticationChallengeReference(r *http.Request) (uuid.UUID, error) {
	defer r.Body.Close()
	var in struct {
		AuthenticationChallengeID string `json:"authentication_challenge_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&in); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(in.AuthenticationChallengeID)
}

func writeAuthenticationChallengeJSONError(w http.ResponseWriter, err error) {
	if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
		response.ErrorJSON(w, http.StatusConflict, response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired.")
		return
	}
	response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Could not update session.")
}

func normalizedReturnTo(raw string, fallback string) string {
	if normalized, ok := redirect.NormalizeReturnTo(raw); ok {
		return normalized
	}
	if normalized, ok := redirect.NormalizeReturnTo(fallback); ok {
		return normalized
	}
	return "/"
}
