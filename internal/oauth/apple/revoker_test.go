package apple

import (
	"context"
	"errors"
	"testing"
)

type fallbackRevokerStub struct {
	calls int
	err   error
}

func (r *fallbackRevokerStub) Revoke(context.Context, string) error {
	r.calls++
	return r.err
}

func TestFallbackRevokerUsesNextClientAfterClientMismatch(t *testing.T) {
	web := &fallbackRevokerStub{err: errors.New("invalid_client")}
	native := &fallbackRevokerStub{}
	if err := NewFallbackRevoker(web, native).Revoke(context.Background(), "native-refresh-token"); err != nil {
		t.Fatal(err)
	}
	if web.calls != 1 || native.calls != 1 {
		t.Fatalf("revoke calls = web %d, native %d", web.calls, native.calls)
	}
}

func TestFallbackRevokerStopsAfterSuccessfulClient(t *testing.T) {
	web := &fallbackRevokerStub{}
	native := &fallbackRevokerStub{}
	if err := NewFallbackRevoker(web, native).Revoke(context.Background(), "web-refresh-token"); err != nil {
		t.Fatal(err)
	}
	if web.calls != 1 || native.calls != 0 {
		t.Fatalf("revoke calls = web %d, native %d", web.calls, native.calls)
	}
}
