package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/http/kit/validation"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

const maxCredentialsBodyBytes = 4096 // ponytail: credential JSON only; raise if this endpoint grows.

type credentialsRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	InvitationCode string `json:"invitation_code,omitempty"`
}

type authResponse struct {
	User         authUser `json:"user"`
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
}

type authUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *APIHandler) SignupPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	in, ok := readCredentials(w, r, SignupPostErrors)
	if !ok {
		return
	}
	if !validation.IsValidEmail(in.Email) || !validation.IsValidPassword(in.Password) {
		response.WriteError(
			w,
			mustRouteError(SignupPostErrors, response.CodeInvalidRequest),
			"Please provide a valid email and password.",
		)
		return
	}
	audience, ok := readAudience(w, r, SignupPostErrors)
	if !ok {
		return
	}
	if audience != token.AudienceApp {
		response.WriteError(
			w,
			mustRouteError(SignupPostErrors, response.CodeForbidden),
			"Signup only supports app audience.",
		)
		return
	}

	allowed, err := h.Limiter.AllowSignupAttempt(ctx, httputil.ClientIP(r), in.Email)
	if err != nil || !allowed {
		response.WriteError(
			w,
			mustRouteError(SignupPostErrors, response.CodeRateLimited),
			"Too many attempts. Please try again later.",
		)
		return
	}

	passwordHash, err := auth.Hash(in.Password)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(SignupPostErrors, response.CodeInternalError),
			"Password error",
		)
		return
	}
	if h.ChallengeEnabled {
		h.startSignupChallenge(w, r, in.Email, passwordHash, in.InvitationCode)
		return
	}

	user, err := h.Auth.Signup(ctx, auth.SignupInput{
		Provider:        domain.ProviderPassword,
		Email:           in.Email,
		PasswordHash:    passwordHash,
		InvitationToken: in.InvitationCode,
	})
	if err != nil {
		code := authSignupErrorCode(err)
		response.WriteError(
			w,
			mustRouteError(SignupPostErrors, code),
			authSignupErrorMessage(err, code),
		)
		return
	}

	h.createSessionResponse(w, r, SignupPostErrors, user, audience, http.StatusCreated)
}

func (h *APIHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	in, ok := readCredentials(w, r, LoginPostErrors)
	if !ok {
		return
	}
	audience, ok := readAudience(w, r, LoginPostErrors)
	if !ok {
		return
	}

	allowed, err := h.Limiter.AllowLoginAttempt(ctx, httputil.ClientIP(r), in.Email)
	if err != nil || !allowed {
		response.WriteError(
			w,
			mustRouteError(LoginPostErrors, response.CodeRateLimited),
			"Too many attempts. Please try again later.",
		)
		return
	}

	user, err := h.Auth.Login(ctx, auth.LoginInput{
		Provider: domain.ProviderPassword,
		Email:    in.Email,
		Password: in.Password,
	})
	if err != nil {
		code := authLoginErrorCode(err)
		message := "Invalid email or password."
		if code == response.CodeInternalError {
			message = "Login error."
		}
		response.WriteError(
			w,
			mustRouteError(LoginPostErrors, code),
			message,
		)
		return
	}

	h.createSessionResponse(w, r, LoginPostErrors, user, audience, http.StatusOK)
}

func readCredentials(
	w http.ResponseWriter,
	r *http.Request,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
) (credentialsRequest, bool) {
	var in credentialsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCredentialsBodyBytes)).Decode(&in); err != nil {
		response.WriteError(
			w,
			mustRouteError(routeErrors, response.CodeInvalidRequest),
			"Invalid JSON body.",
		)
		return credentialsRequest{}, false
	}

	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.InvitationCode = strings.TrimSpace(in.InvitationCode)
	if in.Email == "" || in.Password == "" {
		response.WriteError(
			w,
			mustRouteError(routeErrors, response.CodeInvalidRequest),
			"Email and password required.",
		)
		return credentialsRequest{}, false
	}

	return in, true
}

func authSignupErrorCode(err error) response.ErrorCode {
	switch {
	case errors.Is(err, auth.ErrEmailNotAllowed):
		return response.CodeForbidden
	case errors.Is(err, auth.ErrUserAlreadyExists),
		errors.Is(err, auth.ErrInvalidUsername),
		errors.Is(err, auth.ErrUnsupportedProvider),
		errors.Is(err, store.ErrOrganizationInvitationNotFound),
		errors.Is(err, organization.ErrInvalidOrganizationInvitationToken),
		errors.Is(err, organization.ErrOrganizationInvitationExpired),
		errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted),
		errors.Is(err, organization.ErrOrganizationInvitationRevoked),
		errors.Is(err, organization.ErrOrganizationInviteEmailMismatch),
		errors.Is(err, organization.ErrOrganizationInviteForbidden),
		errors.Is(err, organization.ErrOrganizationSingleMembershipConflict):
		return response.CodeInvalidRequest
	default:
		return response.CodeInternalError
	}
}

func authSignupErrorMessage(err error, code response.ErrorCode) string {
	if code == response.CodeInternalError {
		return "Signup error."
	}
	switch {
	case errors.Is(err, organization.ErrOrganizationInviteEmailMismatch):
		return "Invitation code does not match this email."
	case errors.Is(err, store.ErrOrganizationInvitationNotFound),
		errors.Is(err, organization.ErrInvalidOrganizationInvitationToken):
		return "Invalid invitation code."
	case errors.Is(err, organization.ErrOrganizationInvitationExpired):
		return "This invitation has expired."
	case errors.Is(err, organization.ErrOrganizationInvitationAlreadyAccepted):
		return "This invitation has already been accepted."
	case errors.Is(err, organization.ErrOrganizationInvitationRevoked):
		return "This invitation has been revoked."
	default:
		return "Could not create account. Please check your details."
	}
}

func (h *APIHandler) invitationIDForSignupCode(ctx context.Context, email, invitationCode string) (*uuid.UUID, error) {
	if invitationCode == "" {
		return nil, nil
	}
	if h.Organizations == nil {
		return nil, organization.ErrOrganizationInviteForbidden
	}
	preview, err := h.Organizations.InvitationByToken(ctx, invitationCode)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(strings.TrimSpace(preview.Invitation.Email)) != email {
		return nil, organization.ErrOrganizationInviteEmailMismatch
	}
	switch preview.Invitation.Status(time.Now().UTC()) {
	case domain.OrganizationInvitationStatusPending:
	case domain.OrganizationInvitationStatusAccepted:
		return nil, organization.ErrOrganizationInvitationAlreadyAccepted
	case domain.OrganizationInvitationStatusRevoked:
		return nil, organization.ErrOrganizationInvitationRevoked
	case domain.OrganizationInvitationStatusExpired:
		return nil, organization.ErrOrganizationInvitationExpired
	default:
		return nil, organization.ErrOrganizationInviteForbidden
	}
	id := preview.Invitation.ID
	return &id, nil
}

func authLoginErrorCode(err error) response.ErrorCode {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials),
		errors.Is(err, auth.ErrEmailNotAllowed),
		errors.Is(err, store.ErrUserNotFound),
		errors.Is(err, store.ErrorAuthProviderNotFound):
		return response.CodeUnauthorized
	default:
		return response.CodeInternalError
	}
}

func readAudience(
	w http.ResponseWriter,
	r *http.Request,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
) (token.Audience, bool) {
	audience, err := redirect.AudienceFromRequest(r)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(routeErrors, response.CodeInvalidRequest),
			"Invalid audience.",
		)
		return "", false
	}
	return audience, true
}

func (h *APIHandler) createSessionResponse(
	w http.ResponseWriter,
	r *http.Request,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
	user domain.User,
	audience token.Audience,
	status int,
) {
	accessToken, refreshToken, err := h.Session.CreateSession(
		r.Context(),
		user.ID,
		audience,
		r.UserAgent(),
		time.Now(),
	)
	switch sessionErrorCode(err) {
	case response.CodeForbidden:
		response.WriteError(
			w,
			mustRouteError(routeErrors, response.CodeForbidden),
			"Account cannot access requested audience.",
		)
		return
	case response.CodeInternalError:
		response.WriteError(
			w,
			mustRouteError(routeErrors, response.CodeInternalError),
			"Session error.",
		)
		return
	}

	h.writeSessionResponse(w, user, status, accessToken, refreshToken)
}

func sessionErrorCode(err error) response.ErrorCode {
	if err == nil {
		return ""
	}
	if errors.Is(err, session.ErrForbidden) ||
		errors.Is(err, session.ErrUserDisabled) ||
		errors.Is(err, session.ErrUserNotAllowed) {
		return response.CodeForbidden
	}
	return response.CodeInternalError
}

func (h *APIHandler) writeSessionResponse(
	w http.ResponseWriter,
	user domain.User,
	status int,
	accessToken string,
	refreshToken string,
) {
	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	response.JSON(w, status, authResponse{
		User: authUser{
			ID:        user.ID.String(),
			Email:     user.Email,
			Username:  user.Username,
			Disabled:  user.DisabledAt != nil,
			CreatedAt: user.CreatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}
