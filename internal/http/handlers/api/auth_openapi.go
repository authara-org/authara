package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/http/kit/validation"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
)

func (h *APIHandler) LoginWithPassword(ctx context.Context, request contract.LoginWithPasswordRequestObject) (contract.LoginWithPasswordResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return loginWithPasswordError(responseCodeInternalError(), "API contract error."), nil
	}
	if request.Body == nil {
		return loginWithPasswordError(responseCodeInvalidRequest(), "Invalid JSON body."), nil
	}
	body := request.Body
	identifier := strings.TrimSpace(body.Identifier)
	password := body.Password
	if identifier == "" || password == "" {
		message := "Email and password required."
		if h.UsernameLoginEnabled {
			message = "Email or username and password required."
		}
		return loginWithPasswordError(responseCodeInvalidRequest(), message), nil
	}
	loginInput := auth.LoginInput{
		Provider: domain.ProviderPassword,
		Email:    strings.ToLower(identifier),
		Password: password,
	}
	invalidCredentialsMessage := "Invalid email or password."
	if h.UsernameLoginEnabled {
		loginInput.Identifier = identifier
		loginInput.Email = ""
		invalidCredentialsMessage = "Invalid email, username, or password."
	} else if !validation.IsValidEmail(loginInput.Email) {
		return loginWithPasswordError(responseCodeInvalidRequest(), "Please provide a valid email address."), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	rateLimitIdentifier := strings.ToLower(identifier)
	allowed, err := h.Limiter.AllowLoginAttempt(ctx, httputil.ClientIP(r), rateLimitIdentifier)
	if err != nil || !allowed {
		return loginWithPasswordError(responseCodeRateLimited(), "Too many attempts. Please try again later."), nil
	}
	user, err := h.Auth.Login(ctx, loginInput)
	if err != nil {
		code := authLoginErrorCode(err)
		message := invalidCredentialsMessage
		if code == responseCodeInternalError() {
			message = "Login error."
		}
		return loginWithPasswordError(code, message), nil
	}
	sessionBody, header, code, message, ok := h.contractSession(ctx, r, user, audience)
	if !ok {
		return loginWithPasswordError(code, message), nil
	}
	return contract.LoginWithPassword200HeadersResponse{Header: header, Body: sessionBody}, nil
}

func (h *APIHandler) SignupDirect(ctx context.Context, request contract.SignupDirectRequestObject) (contract.SignupDirectResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return signupDirectError(responseCodeInternalError(), "API contract error."), nil
	}
	if h.ChallengeEnabled {
		return signupDirectError(responseCodeNotFound(), "Direct signup is not enabled."), nil
	}
	in, code, message, ok := signupInputFromBody(request.Body)
	if !ok {
		return signupDirectError(code, message), nil
	}
	audience, code, message, ok := appAudience(request.Params.Audience)
	if !ok {
		return signupDirectError(code, message), nil
	}
	passwordHash, code, message, ok := h.prepareContractSignup(ctx, r, in)
	if !ok {
		return signupDirectError(code, message), nil
	}
	user, err := h.Auth.Signup(ctx, auth.SignupInput{
		Provider:        domain.ProviderPassword,
		Email:           in.Email,
		PasswordHash:    passwordHash,
		InvitationToken: in.InvitationCode,
	})
	if err != nil {
		code := authSignupErrorCode(err)
		return signupDirectError(code, authSignupErrorMessage(err, code)), nil
	}
	body, header, code, message, ok := h.contractSession(ctx, r, user, audience)
	if !ok {
		return signupDirectError(code, message), nil
	}
	return contract.SignupDirect201HeadersResponse{Header: header, Body: body}, nil
}

type contractSignupInput struct {
	Email          string
	Password       string
	InvitationCode string
}

func signupInputFromBody(body *contract.SignupRequest) (contractSignupInput, response.ErrorCode, string, bool) {
	if body == nil {
		return contractSignupInput{}, responseCodeInvalidRequest(), "Invalid JSON body.", false
	}
	in := contractSignupInput{
		Email:    strings.ToLower(strings.TrimSpace(string(body.Email))),
		Password: body.Password,
	}
	if body.InvitationCode != nil {
		in.InvitationCode = strings.TrimSpace(*body.InvitationCode)
	}
	if in.Email == "" || in.Password == "" {
		return contractSignupInput{}, responseCodeInvalidRequest(), "Email and password required.", false
	}
	return in, "", "", true
}

func (h *APIHandler) prepareContractSignup(
	ctx context.Context,
	r *http.Request,
	in contractSignupInput,
) (string, response.ErrorCode, string, bool) {
	if !validationEmailPassword(in.Email, in.Password) {
		return "", responseCodeInvalidRequest(), "Please provide a valid email and password.", false
	}
	allowed, err := h.Limiter.AllowSignupAttempt(ctx, httputil.ClientIP(r), in.Email)
	if err != nil || !allowed {
		return "", responseCodeRateLimited(), "Too many attempts. Please try again later.", false
	}
	passwordHash, err := auth.Hash(in.Password)
	if err != nil {
		return "", responseCodeInternalError(), "Password error", false
	}
	return passwordHash, "", "", true
}

func validationEmailPassword(email, password string) bool {
	return validation.IsValidEmail(email) && validation.IsValidPassword(password)
}

func (h *APIHandler) contractSession(
	ctx context.Context,
	r *http.Request,
	user domain.User,
	audience token.Audience,
) (contract.AuthSession, http.Header, response.ErrorCode, string, bool) {
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, r.UserAgent(), time.Now())
	switch sessionErrorCode(err) {
	case response.CodeForbidden:
		return contract.AuthSession{}, nil, response.CodeForbidden, "Account cannot access requested audience.", false
	case response.CodeInternalError:
		return contract.AuthSession{}, nil, response.CodeInternalError, "Session error.", false
	}
	header := make(http.Header)
	session.SetAccessToken(contract.HeaderWriter(header), accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(contract.HeaderWriter(header), refreshToken, int(h.RefreshTTL.Seconds()))
	return toContractAuthSession(user, accessToken, refreshToken), header, "", "", true
}
