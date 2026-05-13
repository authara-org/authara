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
	ChallengeID  string          `json:"challenge_id"`
	Credential   json.RawMessage `json:"credential"`
	Name         string          `json:"name"`
	PlatformHint string          `json:"platform_hint"`
	ReturnTo     string          `json:"return_to"`
}

const passkeyResponseLinkedProvidersSection = "linked-providers-section"

func (h *UIHandler) PasskeySetupPage(w http.ResponseWriter, r *http.Request) {
	returnTo := httpctx.ReturnToOrDefault(r.Context(), "/")

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

	writeRawJSON(w, http.StatusOK, optionsJSON)
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

	returnTo := normalizedReturnTo(in.ReturnTo, httpctx.ReturnToOrDefault(ctx, "/auth/account"))
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

	writeRawJSON(w, http.StatusOK, optionsJSON)
}

func (h *UIHandler) PasskeyAuthenticateFinishPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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

	now := time.Now().UTC()
	user, err := h.Passkeys.FinishLogin(ctx, challengeID, in.Credential, now)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("passkey login failed", "err", err)
		}
		response.ErrorJSON(w, http.StatusUnprocessableEntity, response.CodeInvalidRequest, "Passkey sign-in failed.")
		return
	}

	returnTo := normalizedReturnTo(in.ReturnTo, httpctx.ReturnToOrDefault(ctx, "/"))
	audience := redirect.AudienceForPath(returnTo)
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, r.UserAgent(), now)
	if err != nil {
		response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Could not create session.")
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	response.JSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"return_to": returnTo,
	})
}

func (h *UIHandler) PasskeyDeletePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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
		passkeys, err = h.Passkeys.ListUserPasskeys(ctx, userID)
		if err != nil {
			return nil, err
		}
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

func writeRawJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
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
