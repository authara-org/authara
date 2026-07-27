package internalapi

import (
	"net/http/httptest"
	"testing"

	contract "github.com/authara-org/authara/internal/http/openapi"
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
