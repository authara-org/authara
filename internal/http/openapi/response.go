package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/response"
)

type requestKey struct{}

// CaptureRequest keeps the original HTTP request available to strict handler
// methods while generated.go passes typed request objects.
func CaptureRequest(next StrictHandlerFunc, _ string) StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		return next(context.WithValue(ctx, requestKey{}, r), w, r, request)
	}
}

func Request(ctx context.Context) (*http.Request, bool) {
	r, ok := ctx.Value(requestKey{}).(*http.Request)
	return r, ok
}

func WithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, requestKey{}, r)
}

type Response struct {
	Status int
	Header http.Header
	Body   any
}

func JSON(status int, body any, headers ...http.Header) Response {
	return Response{Status: status, Header: mergeHeaders(headers...), Body: body}
}

func NoContent(headers ...http.Header) Response {
	return Response{Status: http.StatusNoContent, Header: mergeHeaders(headers...)}
}

func Empty(status int, headers ...http.Header) Response {
	return Response{Status: status, Header: mergeHeaders(headers...)}
}

func ErrorJSON(status int, code response.ErrorCode, message string) Response {
	return JSON(status, ErrorResponse{
		Error: APIError{
			Code:    string(code),
			Message: message,
		},
	})
}

func InternalError() Response {
	return ErrorJSON(http.StatusInternalServerError, response.CodeInternalError, "API contract error.")
}

func HeaderWriter(header http.Header) http.ResponseWriter {
	return headerWriter{header: header}
}

type headerWriter struct {
	header http.Header
}

func (w headerWriter) Header() http.Header {
	return w.header
}

func (headerWriter) WriteHeader(int) {}

func (headerWriter) Write([]byte) (int, error) {
	return 0, nil
}

func mergeHeaders(headers ...http.Header) http.Header {
	out := make(http.Header)
	for _, header := range headers {
		for name, values := range header {
			for _, value := range values {
				out.Add(name, value)
			}
		}
	}
	return out
}

func (r Response) write(w http.ResponseWriter) error {
	for name, values := range r.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	if r.Body == nil {
		w.WriteHeader(r.Status)
		return nil
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.Status)
	_, err := buf.WriteTo(w)
	return err
}

func (r Response) Write(w http.ResponseWriter) error {
	return r.write(w)
}

type GetCsrfToken200HeadersResponse struct {
	Header http.Header
	Body   CSRFToken
}

func (r GetCsrfToken200HeadersResponse) VisitGetCsrfTokenResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type GetGoogleLoginOptions200HeadersResponse struct {
	Header http.Header
	Body   GoogleLoginOptions
}

func (r GetGoogleLoginOptions200HeadersResponse) VisitGetGoogleLoginOptionsResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type LoginWithGoogle200HeadersResponse struct {
	Header http.Header
	Body   AuthSession
}

type LinkCurrentUserGoogle204HeadersResponse struct {
	Header http.Header
}

func (r LinkCurrentUserGoogle204HeadersResponse) VisitLinkCurrentUserGoogleResponse(w http.ResponseWriter) error {
	writeHeaders(w.Header(), r.Header)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (r LoginWithGoogle200HeadersResponse) VisitLoginWithGoogleResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type LoginWithPassword200HeadersResponse struct {
	Header http.Header
	Body   AuthSession
}

type AcceptInvitation200HeadersResponse struct {
	Header http.Header
	Body   Tokens
}

func (r AcceptInvitation200HeadersResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type LoginAndAcceptInvitation200HeadersResponse struct {
	Header http.Header
	Body   AuthSession
}

func (r LoginAndAcceptInvitation200HeadersResponse) VisitLoginAndAcceptInvitationResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type AuthenticateAndAcceptInvitationWithGoogle200HeadersResponse struct {
	Header http.Header
	Body   InvitationGoogleResult
}

func (r AuthenticateAndAcceptInvitationWithGoogle200HeadersResponse) VisitAuthenticateAndAcceptInvitationWithGoogleResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type StartGoogleAccountRecoveryLink202HeadersResponse struct {
	Header http.Header
	Body   AccountRecoveryLink
}

func (r StartGoogleAccountRecoveryLink202HeadersResponse) VisitStartGoogleAccountRecoveryLinkResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusAccepted, r.Header, r.Body)
}

type CompleteAccountRecoveryLinkWithPassword200HeadersResponse struct {
	Header http.Header
	Body   AuthSession
}

func (r CompleteAccountRecoveryLinkWithPassword200HeadersResponse) VisitCompleteAccountRecoveryLinkWithPasswordResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type CompleteAccountRecoveryLinkWithGoogle200HeadersResponse struct {
	Header http.Header
	Body   AuthSession
}

func (r CompleteAccountRecoveryLinkWithGoogle200HeadersResponse) VisitCompleteAccountRecoveryLinkWithGoogleResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

func (r LoginWithPassword200HeadersResponse) VisitLoginWithPasswordResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type FinishPasskeyAuthentication200HeadersResponse struct {
	Header http.Header
	Body   AuthSession
}

func (r FinishPasskeyAuthentication200HeadersResponse) VisitFinishPasskeyAuthenticationResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type SignupDirect201HeadersResponse struct {
	Header http.Header
	Body   AuthSession
}

func (r SignupDirect201HeadersResponse) VisitSignupDirectResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusCreated, r.Header, r.Body)
}

type VerifySignupChallenge201HeadersResponse struct {
	Header http.Header
	Body   AuthSession
}

func (r VerifySignupChallenge201HeadersResponse) VisitVerifySignupChallengeResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusCreated, r.Header, r.Body)
}

type SwitchOrganization200HeadersResponse struct {
	Header http.Header
	Body   Tokens
}

func (r SwitchOrganization200HeadersResponse) VisitSwitchOrganizationResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusOK, r.Header, r.Body)
}

type Logout204HeadersResponse struct {
	Header http.Header
}

func (r Logout204HeadersResponse) VisitLogoutResponse(w http.ResponseWriter) error {
	writeHeaders(w.Header(), r.Header)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type RefreshSession200HeadersResponse struct {
	Header http.Header
}

func (r RefreshSession200HeadersResponse) VisitRefreshSessionResponse(w http.ResponseWriter) error {
	writeHeaders(w.Header(), r.Header)
	w.WriteHeader(http.StatusOK)
	return nil
}

type RefreshSession401HeadersResponse struct {
	Header http.Header
	Body   ErrorResponse
}

func (r RefreshSession401HeadersResponse) VisitRefreshSessionResponse(w http.ResponseWriter) error {
	return writeJSON(w, http.StatusUnauthorized, r.Header, r.Body)
}

func writeJSON(w http.ResponseWriter, status int, header http.Header, body any) error {
	writeHeaders(w.Header(), header)
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := buf.WriteTo(w)
	return err
}

func writeHeaders(dst, src http.Header) {
	for name, values := range src {
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}
