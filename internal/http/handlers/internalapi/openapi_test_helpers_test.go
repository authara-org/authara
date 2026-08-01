package internalapi

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
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
