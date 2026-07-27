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

func (r Response) VisitBeginPasskeyAuthenticationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitBeginPasskeyRegistrationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitCreateInternalOrganizationInvitationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitCreateInternalOrganizationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitFinishPasskeyAuthenticationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitFinishPasskeyRegistrationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitGetCsrfTokenResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitGetCurrentOrganizationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitGetCurrentUserResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitGetGoogleLoginOptionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitGetPublicCapabilitiesResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitGetPublicOrganizationInvitationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitGetPublicOrganizationMemberResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitGetPublicOrganizationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitListCurrentOrganizationMembersResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitListCurrentUserOrganizationsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitListPublicOrganizationInvitationsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitListPublicOrganizationMembersResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitListPublicUserMembershipsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitLoginWithGoogleResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitLoginWithPasswordResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitLogoutResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitRefreshSessionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitRefreshTokensResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitResendChallengeResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitResendInternalOrganizationInvitationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitRevokePublicOrganizationInvitationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitSignupDirectResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitStartSignupChallengeResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitSwitchOrganizationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitUpdatePublicOrganizationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r Response) VisitVerifySignupChallengeResponse(w http.ResponseWriter) error {
	return r.write(w)
}
