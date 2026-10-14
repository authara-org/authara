package oauthstate

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPendingNoncesAreConsumedIndependently(t *testing.T) {
	Configure(false)
	t.Cleanup(func() { Configure(true) })

	firstResponse := httptest.NewRecorder()
	first, err := Create(firstResponse, httptest.NewRequest(http.MethodGet, "/auth", nil))
	if err != nil {
		t.Fatal(err)
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/auth", nil)
	secondRequest.AddCookie(firstResponse.Result().Cookies()[0])
	secondResponse := httptest.NewRecorder()
	second, err := Create(secondResponse, secondRequest)
	if err != nil {
		t.Fatal(err)
	}

	consumeFirst := httptest.NewRequest(http.MethodPost, "/auth", nil)
	consumeFirst.AddCookie(secondResponse.Result().Cookies()[0])
	firstResult := httptest.NewRecorder()
	if got, ok := Consume(firstResult, consumeFirst, first); !ok || got != first {
		t.Fatalf("Consume(first) = %q, %t", got, ok)
	}
	remaining := firstResult.Result().Cookies()[0]
	if remaining.MaxAge < 0 {
		t.Fatal("consuming the first nonce cleared the second")
	}

	consumeSecond := httptest.NewRequest(http.MethodPost, "/auth", nil)
	consumeSecond.AddCookie(remaining)
	secondResult := httptest.NewRecorder()
	if got, ok := Consume(secondResult, consumeSecond, second); !ok || got != second {
		t.Fatalf("Consume(second) = %q, %t", got, ok)
	}
	if cookie := secondResult.Result().Cookies()[0]; cookie.MaxAge != -1 {
		t.Fatalf("last nonce cookie MaxAge = %d, want -1", cookie.MaxAge)
	}
}

func TestConsumeRejectsReplayAndPreservesOtherNonce(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/auth", nil)
	request.AddCookie(&http.Cookie{Name: CookieName, Value: "first|second"})
	response := httptest.NewRecorder()
	if _, ok := Consume(response, request, "missing"); ok {
		t.Fatal("Consume accepted a missing nonce")
	}
	if response.Header().Get("Set-Cookie") != "" {
		t.Fatal("rejected nonce changed pending flows")
	}
}

func TestSimultaneousCreatesRemainIndependentlyConsumable(t *testing.T) {
	Configure(false)
	t.Cleanup(func() { Configure(true) })

	request := httptest.NewRequest(http.MethodGet, "/auth", nil)
	firstResponse := httptest.NewRecorder()
	first, err := Create(firstResponse, request)
	if err != nil {
		t.Fatal(err)
	}
	secondResponse := httptest.NewRecorder()
	second, err := Create(secondResponse, request)
	if err != nil {
		t.Fatal(err)
	}

	consume := httptest.NewRequest(http.MethodPost, "/auth", nil)
	for _, cookie := range secondResponse.Result().Cookies() {
		consume.AddCookie(cookie)
	}
	for _, cookie := range firstResponse.Result().Cookies() {
		if cookie.Name == flowCookieName(first) {
			consume.AddCookie(cookie)
		}
	}
	result := httptest.NewRecorder()
	if got, ok := Consume(result, consume, first); !ok || got != first {
		t.Fatalf("Consume(first) = %q, %t; second nonce was %q", got, ok, second)
	}
}
