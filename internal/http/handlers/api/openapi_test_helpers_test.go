package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/session/token"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func writeContractResponse(t *testing.T, rr *httptest.ResponseRecorder, resp any) {
	t.Helper()

	out, ok := resp.(contract.Response)
	if !ok {
		t.Fatalf("expected contract.Response, got %T", resp)
	}
	if err := out.Write(rr); err != nil {
		t.Fatalf("write contract response: %v", err)
	}
}

func contractCtx(ctx context.Context, r *http.Request) context.Context {
	return contract.WithRequest(ctx, r)
}

func signupRequest(email, password, invitationCode string) *contract.SignupRequest {
	req := &contract.SignupRequest{
		Email:    openapi_types.Email(email),
		Password: password,
	}
	if invitationCode != "" {
		req.InvitationCode = &invitationCode
	}
	return req
}

func passwordLoginRequest(email, password string) *contract.PasswordLoginRequest {
	return &contract.PasswordLoginRequest{
		Email:    openapi_types.Email(email),
		Password: password,
	}
}

func appAudienceParam() *contract.Audience {
	audience := contract.Audience(token.AudienceApp)
	return &audience
}
