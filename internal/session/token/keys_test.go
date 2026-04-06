package token

import (
	"bytes"
	"testing"
)

func TestNewKeySet_Succeeds(t *testing.T) {
	keys := map[string][]byte{
		"key-1": []byte("super-secret-key-1"),
		"key-2": []byte("super-secret-key-2"),
	}

	ks, err := NewKeySet("key-1", keys)
	if err != nil {
		t.Fatalf("NewKeySet returned error: %v", err)
	}
	if ks == nil {
		t.Fatal("expected non-nil KeySet")
	}

	if ks.ActiveKeyID != "key-1" {
		t.Fatalf("expected ActiveKeyID %q, got %q", "key-1", ks.ActiveKeyID)
	}

	gotID, gotKey := ks.SigningKey()
	if gotID != "key-1" {
		t.Fatalf("expected signing key id %q, got %q", "key-1", gotID)
	}
	if !bytes.Equal(gotKey, []byte("super-secret-key-1")) {
		t.Fatalf("unexpected signing key: got %q", string(gotKey))
	}
}

func TestNewKeySet_EmptyActiveKeyID(t *testing.T) {
	keys := map[string][]byte{
		"key-1": []byte("super-secret-key-1"),
	}

	ks, err := NewKeySet("", keys)
	if err == nil {
		t.Fatal("expected error for empty active key id")
	}
	if err.Error() != "active key id must be set" {
		t.Fatalf("unexpected error: %v", err)
	}
	if ks != nil {
		t.Fatal("expected nil KeySet on error")
	}
}

func TestNewKeySet_ActiveKeyMissing(t *testing.T) {
	keys := map[string][]byte{
		"key-1": []byte("super-secret-key-1"),
	}

	ks, err := NewKeySet("missing-key", keys)
	if err == nil {
		t.Fatal("expected error for missing active key")
	}
	if err.Error() != "active signing key not found" {
		t.Fatalf("unexpected error: %v", err)
	}
	if ks != nil {
		t.Fatal("expected nil KeySet on error")
	}
}

func TestNewKeySet_ActiveKeyEmptyValue(t *testing.T) {
	keys := map[string][]byte{
		"key-1": {},
	}

	ks, err := NewKeySet("key-1", keys)
	if err == nil {
		t.Fatal("expected error for empty active signing key")
	}
	if err.Error() != "active signing key not found" {
		t.Fatalf("unexpected error: %v", err)
	}
	if ks != nil {
		t.Fatal("expected nil KeySet on error")
	}
}

func TestNewKeySet_ActiveKeyNilValue(t *testing.T) {
	keys := map[string][]byte{
		"key-1": nil,
	}

	ks, err := NewKeySet("key-1", keys)
	if err == nil {
		t.Fatal("expected error for nil active signing key")
	}
	if err.Error() != "active signing key not found" {
		t.Fatalf("unexpected error: %v", err)
	}
	if ks != nil {
		t.Fatal("expected nil KeySet on error")
	}
}

func TestSigningKey_ReturnsActiveKey(t *testing.T) {
	keys := map[string][]byte{
		"key-1": []byte("secret-1"),
		"key-2": []byte("secret-2"),
	}

	ks, err := NewKeySet("key-2", keys)
	if err != nil {
		t.Fatalf("NewKeySet returned error: %v", err)
	}

	gotID, gotKey := ks.SigningKey()
	if gotID != "key-2" {
		t.Fatalf("expected signing key id %q, got %q", "key-2", gotID)
	}
	if !bytes.Equal(gotKey, []byte("secret-2")) {
		t.Fatalf("expected signing key %q, got %q", "secret-2", string(gotKey))
	}
}

func TestVerificationKey_Found(t *testing.T) {
	keys := map[string][]byte{
		"key-1": []byte("secret-1"),
		"key-2": []byte("secret-2"),
	}

	ks, err := NewKeySet("key-1", keys)
	if err != nil {
		t.Fatalf("NewKeySet returned error: %v", err)
	}

	gotKey, ok := ks.VerificationKey("key-2")
	if !ok {
		t.Fatal("expected verification key to be found")
	}
	if !bytes.Equal(gotKey, []byte("secret-2")) {
		t.Fatalf("expected verification key %q, got %q", "secret-2", string(gotKey))
	}
}

func TestVerificationKey_NotFound(t *testing.T) {
	keys := map[string][]byte{
		"key-1": []byte("secret-1"),
	}

	ks, err := NewKeySet("key-1", keys)
	if err != nil {
		t.Fatalf("NewKeySet returned error: %v", err)
	}

	gotKey, ok := ks.VerificationKey("missing-key")
	if ok {
		t.Fatal("expected missing verification key to return ok=false")
	}
	if gotKey != nil {
		t.Fatalf("expected nil key for missing verification key, got %v", gotKey)
	}
}

func TestVerificationKey_FoundButEmptyValue(t *testing.T) {
	keys := map[string][]byte{
		"key-1": []byte("secret-1"),
		"key-2": {},
	}

	ks, err := NewKeySet("key-1", keys)
	if err != nil {
		t.Fatalf("NewKeySet returned error: %v", err)
	}

	gotKey, ok := ks.VerificationKey("key-2")
	if !ok {
		t.Fatal("expected key to be found")
	}
	if len(gotKey) != 0 {
		t.Fatalf("expected empty key, got %v", gotKey)
	}
}
