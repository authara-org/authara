package webhook

import "testing"

func TestSign_Deterministic(t *testing.T) {
	secret := "super-secret"
	body := []byte(`{"event":"user.created","data":{"user_id":"123"}}`)

	got1 := Sign(secret, body)
	got2 := Sign(secret, body)

	if got1 == "" {
		t.Fatal("expected non-empty signature")
	}
	if got1 != got2 {
		t.Fatalf("expected deterministic signature, got %q and %q", got1, got2)
	}
	if got1[:7] != "sha256=" {
		t.Fatalf("expected sha256= prefix, got %q", got1)
	}
}

func TestSign_ChangesWhenBodyChanges(t *testing.T) {
	secret := "super-secret"

	got1 := Sign(secret, []byte(`{"event":"user.created"}`))
	got2 := Sign(secret, []byte(`{"event":"user.deleted"}`))

	if got1 == got2 {
		t.Fatal("expected different signatures for different bodies")
	}
}

func TestSign_ChangesWhenSecretChanges(t *testing.T) {
	body := []byte(`{"event":"user.created"}`)

	got1 := Sign("secret-a", body)
	got2 := Sign("secret-b", body)

	if got1 == got2 {
		t.Fatal("expected different signatures for different secrets")
	}
}
