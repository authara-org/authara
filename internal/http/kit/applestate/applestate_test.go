package applestate

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateConsumeAndClear(t *testing.T) {
	Configure(false)
	t.Cleanup(func() { Configure(true) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/auth/api/v1/oauth/apple/options", nil)
	flow, err := Create(recorder, request)
	if err != nil {
		t.Fatal(err)
	}
	if flow.State == "" || flow.Nonce == "" || flow.State == flow.Nonce {
		t.Fatalf("unexpected flow: %+v", flow)
	}

	request = httptest.NewRequest(http.MethodGet, "/auth/api/v1/oauth/apple", nil)
	request.AddCookie(recorder.Result().Cookies()[0])
	consumed := httptest.NewRecorder()
	got, ok := Consume(consumed, request, flow.State)
	if !ok || got != flow {
		t.Fatalf("Consume() = %+v, %v; want %+v, true", got, ok, flow)
	}
	if !hasClearedCookie(consumed.Result().Cookies(), CookieName) {
		t.Fatal("consumed flow cookie was not cleared")
	}

	clear := httptest.NewRecorder()
	Clear(clear)
	cookie := clear.Result().Cookies()[0]
	if cookie.Name != CookieName || cookie.MaxAge != -1 || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("clear cookie = %+v", cookie)
	}
}

func TestConcurrentFlowsAreConsumedIndependently(t *testing.T) {
	Configure(false)
	t.Cleanup(func() { Configure(true) })

	firstResponse := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/auth/api/v1/oauth/apple/options", nil)
	first, err := Create(firstResponse, firstRequest)
	if err != nil {
		t.Fatal(err)
	}

	secondResponse := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodGet, "/auth/api/v1/oauth/apple/options", nil)
	secondRequest.AddCookie(firstResponse.Result().Cookies()[0])
	second, err := Create(secondResponse, secondRequest)
	if err != nil {
		t.Fatal(err)
	}

	consumeFirst := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/apple", nil)
	consumeFirst.AddCookie(secondResponse.Result().Cookies()[0])
	firstResult := httptest.NewRecorder()
	if got, ok := Consume(firstResult, consumeFirst, first.State); !ok || got != first {
		t.Fatalf("first Consume() = %+v, %v", got, ok)
	}
	remaining := firstResult.Result().Cookies()[0]
	if remaining.MaxAge < 0 {
		t.Fatal("consuming the first flow cleared the second flow")
	}

	consumeSecond := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/apple", nil)
	consumeSecond.AddCookie(remaining)
	secondResult := httptest.NewRecorder()
	if got, ok := Consume(secondResult, consumeSecond, second.State); !ok || got != second {
		t.Fatalf("second Consume() = %+v, %v", got, ok)
	}
	if !hasClearedCookie(secondResult.Result().Cookies(), CookieName) {
		t.Fatal("last consumed flow cookie was not cleared")
	}
}

func hasClearedCookie(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.MaxAge < 0 {
			return true
		}
	}
	return false
}
