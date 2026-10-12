package csrf

import (
	"net/http/httptest"
	"testing"
)

func TestEnsureCookieConfiguredDomain(t *testing.T) {
	Configure(true, "example.com")
	defer Configure(true, "")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "https://auth.example.com/auth/api/v1/csrf", nil)
	if _, err := EnsureCookie(recorder, request); err != nil {
		t.Fatalf("ensure cookie: %v", err)
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Domain != "example.com" {
		t.Fatalf("cookies = %#v, want domain example.com", cookies)
	}
}
