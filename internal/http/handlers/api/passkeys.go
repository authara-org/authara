package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/google/uuid"
)

const maxPasskeyBodyBytes = 128 * 1024

type passkeyFinishRequest struct {
	ChallengeID  string          `json:"challenge_id"`
	Credential   json.RawMessage `json:"credential"`
	Name         string          `json:"name"`
	PlatformHint string          `json:"platform_hint"`
}

func (h *APIHandler) PasskeyAuthenticateOptionsPost(w http.ResponseWriter, r *http.Request) {
	if h.Passkeys == nil {
		writePasskeyInternalError(w, PasskeyAuthenticateOptionsPostErrors)
		return
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginAttempt(r.Context(), httputil.ClientIP(r))
		if err != nil || !allowed {
			response.WriteError(
				w,
				mustRouteError(PasskeyAuthenticateOptionsPostErrors, response.CodeRateLimited),
				"Too many attempts. Please try again later.",
			)
			return
		}
	}

	optionsJSON, _, err := h.Passkeys.BeginLogin(r.Context())
	if err != nil {
		writePasskeyInternalError(w, PasskeyAuthenticateOptionsPostErrors)
		return
	}

	response.RawJSON(w, http.StatusOK, optionsJSON)
}

func (h *APIHandler) PasskeyAuthenticateFinishPost(w http.ResponseWriter, r *http.Request) {
	if h.Passkeys == nil {
		writePasskeyInternalError(w, PasskeyAuthenticateFinishPostErrors)
		return
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginFinishAttempt(r.Context(), httputil.ClientIP(r))
		if err != nil || !allowed {
			response.WriteError(
				w,
				mustRouteError(PasskeyAuthenticateFinishPostErrors, response.CodeRateLimited),
				"Too many attempts. Please try again later.",
			)
			return
		}
	}

	in, err := decodePasskeyFinishRequest(w, r)
	if err != nil {
		writeInvalidPasskeyRequest(w, PasskeyAuthenticateFinishPostErrors)
		return
	}
	challengeID, err := uuid.Parse(in.ChallengeID)
	if err != nil {
		writeInvalidPasskeyRequest(w, PasskeyAuthenticateFinishPostErrors)
		return
	}
	audience, ok := readAudience(w, r, PasskeyAuthenticateFinishPostErrors)
	if !ok {
		return
	}

	user, err := h.Passkeys.FinishLogin(r.Context(), challengeID, in.Credential, time.Now().UTC())
	if err != nil {
		if errors.Is(err, passkey.ErrPasskeyAuthenticationInvalid) {
			response.WriteError(
				w,
				mustRouteError(PasskeyAuthenticateFinishPostErrors, response.CodeUnauthorized),
				"Passkey sign-in failed.",
			)
			return
		}
		writePasskeyInternalError(w, PasskeyAuthenticateFinishPostErrors)
		return
	}

	h.createSessionResponse(w, r, PasskeyAuthenticateFinishPostErrors, user, audience, http.StatusOK)
}

func (h *APIHandler) PasskeyRegisterOptionsPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		response.WriteError(
			w,
			mustRouteError(PasskeyRegisterOptionsPostErrors, response.CodeUnauthorized),
			"Unauthorized.",
		)
		return
	}
	if h.Passkeys == nil {
		writePasskeyInternalError(w, PasskeyRegisterOptionsPostErrors)
		return
	}

	optionsJSON, _, err := h.Passkeys.BeginRegistration(r.Context(), userID)
	if err != nil {
		writePasskeyInternalError(w, PasskeyRegisterOptionsPostErrors)
		return
	}

	response.RawJSON(w, http.StatusOK, optionsJSON)
}

func (h *APIHandler) PasskeyRegisterFinishPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		response.WriteError(
			w,
			mustRouteError(PasskeyRegisterFinishPostErrors, response.CodeUnauthorized),
			"Unauthorized.",
		)
		return
	}
	if h.Passkeys == nil {
		writePasskeyInternalError(w, PasskeyRegisterFinishPostErrors)
		return
	}

	in, err := decodePasskeyFinishRequest(w, r)
	if err != nil {
		writeInvalidPasskeyRequest(w, PasskeyRegisterFinishPostErrors)
		return
	}
	challengeID, err := uuid.Parse(in.ChallengeID)
	if err != nil {
		writeInvalidPasskeyRequest(w, PasskeyRegisterFinishPostErrors)
		return
	}

	err = h.Passkeys.FinishRegistration(
		r.Context(),
		userID,
		challengeID,
		in.Credential,
		passkey.RegistrationMetadata{
			Name:         in.Name,
			UserAgent:    r.UserAgent(),
			PlatformHint: in.PlatformHint,
		},
	)
	switch {
	case errors.Is(err, passkey.ErrPasskeyAlreadyExists):
		response.WriteError(
			w,
			mustRouteError(PasskeyRegisterFinishPostErrors, codePasskeyAlreadyExists),
			"This passkey is already linked to an account.",
		)
		return
	case errors.Is(err, passkey.ErrPasskeyRegistrationInvalid):
		response.WriteError(
			w,
			mustRouteError(PasskeyRegisterFinishPostErrors, codePasskeyRegistrationInvalid),
			"Passkey setup could not be verified.",
		)
		return
	case err != nil:
		writePasskeyInternalError(w, PasskeyRegisterFinishPostErrors)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func decodePasskeyFinishRequest(w http.ResponseWriter, r *http.Request) (passkeyFinishRequest, error) {
	defer r.Body.Close()

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPasskeyBodyBytes))
	var in passkeyFinishRequest
	if err := decoder.Decode(&in); err != nil {
		return passkeyFinishRequest{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return passkeyFinishRequest{}, errors.New("request body must contain one JSON value")
	}

	in.ChallengeID = strings.TrimSpace(in.ChallengeID)
	if in.ChallengeID == "" || len(in.Credential) == 0 || string(in.Credential) == "null" {
		return passkeyFinishRequest{}, errors.New("missing challenge or credential")
	}
	return in, nil
}

func writeInvalidPasskeyRequest(
	w http.ResponseWriter,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
) {
	response.WriteError(
		w,
		mustRouteError(routeErrors, response.CodeInvalidRequest),
		"Invalid passkey response.",
	)
}

func writePasskeyInternalError(
	w http.ResponseWriter,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
) {
	response.WriteError(
		w,
		mustRouteError(routeErrors, response.CodeInternalError),
		"Passkey error.",
	)
}
