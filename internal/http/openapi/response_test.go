package openapi

import "testing"

func TestGenericResponseDoesNotSatisfyGeneratedResponses(t *testing.T) {
	if _, ok := any(Response{}).(GetCsrfTokenResponseObject); ok {
		t.Fatal("generic Response must not satisfy generated operation response interfaces")
	}
}
