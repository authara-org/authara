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
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	if request.Body == nil {
		return routeError(LoginWithPasswordErrors, responseCodeInvalidRequest(), "Invalid JSON body."), nil
	}
	body := request.Body
	email := strings.ToLower(strings.TrimSpace(string(body.Email)))
	password := body.Password
	if email == "" || password == "" {
		return routeError(LoginWithPasswordErrors, responseCodeInvalidRequest(), "Email and password required."), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	allowed, err := h.Limiter.AllowLoginAttempt(ctx, httputil.ClientIP(r), email)
	if err != nil || !allowed {
		return routeError(LoginWithPasswordErrors, responseCodeRateLimited(), "Too many attempts. Please try again later."), nil
	}
	user, err := h.Auth.Login(ctx, auth.LoginInput{
		Provider: domain.ProviderPassword,
		Email:    email,
		Password: password,
	})
	if err != nil {
		code := authLoginErrorCode(err)
		message := "Invalid email or password."
		if code == responseCodeInternalError() {
			message = "Login error."
		}
		return routeError(LoginWithPasswordErrors, code, message), nil
	}
	return h.contractSessionResponse(ctx, r, LoginWithPasswordErrors, user, audience, http.StatusOK), nil
}

func (h *APIHandler) SignupDirect(ctx context.Context, request contract.SignupDirectRequestObject) (contract.SignupDirectResponseObject, error) {
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	if h.ChallengeEnabled {
		return routeError(SignupDirectErrors, responseCodeNotFound(), "Direct signup is not enabled."), nil
	}
	in, out, ok := signupInputFromBody(request.Body, SignupDirectErrors)
	if !ok {
		return out, nil
	}
	audience, out, ok := appAudience(request.Params.Audience, SignupDirectErrors)
	if !ok {
		return out, nil
	}
	passwordHash, out, ok := h.prepareContractSignup(ctx, r, in, SignupDirectErrors)
	if !ok {
		return out, nil
	}
	user, err := h.Auth.Signup(ctx, auth.SignupInput{
		Provider:        domain.ProviderPassword,
		Email:           in.Email,
		PasswordHash:    passwordHash,
		InvitationToken: in.InvitationCode,
	})
	if err != nil {
		code := authSignupErrorCode(err)
		return routeError(SignupDirectErrors, code, authSignupErrorMessage(err, code)), nil
	}
	return h.contractSessionResponse(ctx, r, SignupDirectErrors, user, audience, http.StatusCreated), nil
}

type contractSignupInput struct {
	Email          string
	Password       string
	InvitationCode string
}

func signupInputFromBody(body *contract.SignupRequest, routeErrors map[response.ErrorCode]response.ErrorSpec) (contractSignupInput, contract.Response, bool) {
	if body == nil {
		return contractSignupInput{}, routeError(routeErrors, responseCodeInvalidRequest(), "Invalid JSON body."), false
	}
	in := contractSignupInput{
		Email:    strings.ToLower(strings.TrimSpace(string(body.Email))),
		Password: body.Password,
	}
	if body.InvitationCode != nil {
		in.InvitationCode = strings.TrimSpace(*body.InvitationCode)
	}
	if in.Email == "" || in.Password == "" {
		return contractSignupInput{}, routeError(routeErrors, responseCodeInvalidRequest(), "Email and password required."), false
	}
	return in, contract.Response{}, true
}

func (h *APIHandler) prepareContractSignup(
	ctx context.Context,
	r *http.Request,
	in contractSignupInput,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
) (string, contract.Response, bool) {
	if !validationEmailPassword(in.Email, in.Password) {
		return "", routeError(routeErrors, responseCodeInvalidRequest(), "Please provide a valid email and password."), false
	}
	allowed, err := h.Limiter.AllowSignupAttempt(ctx, httputil.ClientIP(r), in.Email)
	if err != nil || !allowed {
		return "", routeError(routeErrors, responseCodeRateLimited(), "Too many attempts. Please try again later."), false
	}
	passwordHash, err := auth.Hash(in.Password)
	if err != nil {
		return "", routeError(routeErrors, responseCodeInternalError(), "Password error"), false
	}
	return passwordHash, contract.Response{}, true
}

func validationEmailPassword(email, password string) bool {
	return validation.IsValidEmail(email) && validation.IsValidPassword(password)
}

func (h *APIHandler) contractSessionResponse(
	ctx context.Context,
	r *http.Request,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
	user domain.User,
	audience token.Audience,
	status int,
) contract.Response {
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, r.UserAgent(), time.Now())
	switch sessionErrorCode(err) {
	case response.CodeForbidden:
		return routeError(routeErrors, response.CodeForbidden, "Account cannot access requested audience.")
	case response.CodeInternalError:
		return routeError(routeErrors, response.CodeInternalError, "Session error.")
	}
	header := make(http.Header)
	session.SetAccessToken(contract.HeaderWriter(header), accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(contract.HeaderWriter(header), refreshToken, int(h.RefreshTTL.Seconds()))
	return contract.JSON(status, toContractAuthSession(user, accessToken, refreshToken), header)
}
