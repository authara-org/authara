package ui

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/validation"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/identity"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/google/uuid"
)

func (h *UIHandler) EmailVerificationPage(w http.ResponseWriter, r *http.Request) {
	transactionID, ok := session.ReadEmailVerificationTransaction(r)
	if !ok {
		redirect.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}
	transaction, user, err := h.Challenge.GetEmailVerificationTransaction(r.Context(), transactionID, time.Now().UTC())
	if err != nil {
		session.ClearEmailVerificationTransaction(w)
		redirect.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}
	targetEmail, challengeID := "", ""
	if transaction.TargetEmail != nil {
		targetEmail = *transaction.TargetEmail
	}
	if transaction.ChallengeID != nil {
		challengeID = transaction.ChallengeID.String()
	}
	_ = h.Render(w, r, http.StatusOK, authview.EmailVerification(user.Email, targetEmail, challengeID))
}

func (h *UIHandler) EmailVerificationStartPost(w http.ResponseWriter, r *http.Request) {
	transactionID, ok := session.ReadEmailVerificationTransaction(r)
	if !ok {
		redirect.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderInternalError(w, r)
		return
	}
	targetEmail := identity.CanonicalEmail(r.FormValue("email"))
	if !validation.IsValidEmail(targetEmail) {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Please provide a valid email address.", authview.EmailVerification("", targetEmail, ""))
		return
	}
	challengeID, err := h.Challenge.CreateRequiredEmailVerificationChallenge(r.Context(), transactionID, targetEmail, time.Now().UTC())
	if err != nil {
		message := "Could not start email verification. Please try again."
		if errors.Is(err, challenge.ErrEmailAlreadyInUse) {
			message = "That email address is already in use."
		}
		h.renderFormError(w, r, http.StatusUnprocessableEntity, message, authview.EmailVerification("", targetEmail, ""))
		return
	}
	_, user, err := h.Challenge.GetEmailVerificationTransaction(r.Context(), transactionID, time.Now().UTC())
	if err != nil {
		h.renderInternalError(w, r)
		return
	}
	_ = h.Render(w, r, http.StatusOK, authview.EmailVerification(user.Email, targetEmail, challengeID.String()))
}

func (h *UIHandler) EmailVerificationCompletePost(w http.ResponseWriter, r *http.Request) {
	transactionID, ok := session.ReadEmailVerificationTransaction(r)
	if !ok {
		redirect.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderInternalError(w, r)
		return
	}
	challengeID, err := uuid.Parse(strings.TrimSpace(r.FormValue("challenge_id")))
	code := strings.TrimSpace(r.FormValue("code"))
	if err != nil || len(code) != 6 {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Enter the 6-digit verification code.", authview.EmailVerification("", "", r.FormValue("challenge_id")))
		return
	}
	now := time.Now().UTC()
	transaction, user, err := h.Challenge.CompleteRequiredEmailVerification(r.Context(), transactionID, challengeID, code, h.Verification, now)
	if err != nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, h.verifyChallengeErrorMessage(err), authview.EmailVerification("", "", challengeID.String()))
		return
	}
	audience := token.Audience(transaction.Audience)
	accessToken, refreshToken, err := h.Session.CreateSession(r.Context(), user.ID, audience, transaction.AuthenticationMethod, r.UserAgent(), now, httputil.ClientIPString(r))
	if err != nil {
		h.renderInternalError(w, r)
		return
	}
	cookiePolicy := h.sessionCookiePolicy()
	session.SetAccessToken(w, accessToken, int(cookiePolicy.AccessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(cookiePolicy.RefreshTokenTTL.Seconds()))
	session.ClearEmailVerificationTransaction(w)
	redirect.Redirect(w, r, transaction.ReturnTo, http.StatusSeeOther)
}
