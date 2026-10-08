package ui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/applestate"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/google/uuid"
)

type appleProviderProofRequest struct {
	Code   string `json:"code"`
	State  string `json:"state"`
	LinkID string `json:"link_id"`
}

func (h *UIHandler) AppleProviderProofPost(w http.ResponseWriter, r *http.Request) {
	if h.Apple == nil {
		response.ErrorJSON(w, http.StatusNotFound, response.CodeNotFound, "Apple authentication is not enabled.")
		return
	}

	var body appleProviderProofRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 32<<10))
	if err := decoder.Decode(&body); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid Apple authentication request.")
		return
	}
	body.Code = strings.TrimSpace(body.Code)
	body.State = strings.TrimSpace(body.State)
	linkID, err := uuid.Parse(strings.TrimSpace(body.LinkID))
	if err != nil || body.Code == "" || body.State == "" {
		response.ErrorJSON(w, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid Apple authentication request.")
		return
	}

	flow, ok := applestate.Consume(w, r, body.State)
	if !ok {
		h.recordAppleProofDenial(r.Context())
		response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Invalid Apple authentication.")
		return
	}
	result, err := h.Apple.Exchange(r.Context(), body.Code, flow.Nonce)
	if err != nil || result.Identity.OAuthID == "" {
		h.discardAppleProofAuthorization(r.Context(), result.RefreshToken)
		h.recordAppleProofDenial(r.Context())
		response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Invalid Apple authentication.")
		return
	}

	link, err := h.Auth.GetPendingProviderLink(r.Context(), linkID)
	if err != nil {
		h.discardAppleProofAuthorization(r.Context(), result.RefreshToken)
		response.ErrorJSON(w, http.StatusUnprocessableEntity, response.CodeInvalidRequest, "Invalid or expired account connection request.")
		return
	}
	user, err := h.Auth.CompleteAccountRecoveryProviderLinkWithProviderProof(
		r.Context(),
		linkID,
		domain.ProviderApple,
		result.Identity.OAuthID,
		time.Now().UTC(),
	)
	if err != nil {
		h.discardAppleProofAuthorization(r.Context(), result.RefreshToken)
		response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Apple could not verify this account.")
		return
	}

	if result.RefreshToken != "" {
		if err := h.Auth.SaveAppleCredential(r.Context(), user.ID, result.RefreshToken); err != nil {
			h.discardAppleProofAuthorization(r.Context(), result.RefreshToken)
			if h.Logger != nil {
				h.Logger.Error("could not refresh Apple credential after account proof", "user_id", user.ID, "err", err)
			}
		}
	}

	h.finishAccountRecoveryProviderProof(w, r, user, link.Provider, domain.AuthenticationMethodApple)
}

func (h *UIHandler) recordAppleProofDenial(ctx context.Context) {
	if err := h.Auth.RecordLoginDenied(ctx, domain.AuthenticationMethodApple, domain.SecurityEventReasonInvalidAssertion); err != nil && h.Logger != nil {
		h.Logger.Error("record Apple account-proof denial", "err", err)
	}
}

func (h *UIHandler) discardAppleProofAuthorization(ctx context.Context, refreshToken string) {
	if refreshToken == "" {
		return
	}
	if h.AppleCredentials != nil {
		queueCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := h.AppleCredentials.QueueRevocation(queueCtx, refreshToken)
		cancel()
		if err == nil {
			return
		}
		if h.Logger != nil {
			h.Logger.Error("could not queue Apple proof authorization revocation", "err", err)
		}
	}
	revokeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	err := h.Apple.Revoke(revokeCtx, refreshToken)
	cancel()
	if err != nil && h.Logger != nil {
		h.Logger.Error("could not revoke unused Apple proof authorization", "err", err)
	}
}
