package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/session/token"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func writeContractResponse(t *testing.T, rr *httptest.ResponseRecorder, resp any) {
	t.Helper()

	value := reflect.ValueOf(resp)
	for i := 0; i < value.NumMethod(); i++ {
		methodType := value.Type().Method(i)
		if strings.HasPrefix(methodType.Name, "Visit") && strings.HasSuffix(methodType.Name, "Response") {
			out := value.Method(i).Call([]reflect.Value{reflect.ValueOf(rr)})
			if len(out) == 1 && !out[0].IsNil() {
				t.Fatalf("write contract response: %v", out[0].Interface())
			}
			return
		}
	}
	t.Fatalf("expected generated contract response, got %T", resp)
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
