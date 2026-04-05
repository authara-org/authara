package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	password := "super-secret-password"

	encoded, err := Hash(password)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	ok, err := Verify(password, encoded)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected password verification to succeed")
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	encoded, err := Hash("correct-password")
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	ok, err := Verify("wrong-password", encoded)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected password verification to fail")
	}
}

func TestHashUsesRandomSalt(t *testing.T) {
	password := "same-password"

	hash1, err := Hash(password)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	hash2, err := Hash(password)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	if hash1 == hash2 {
		t.Fatalf("expected hashes to differ due to random salt")
	}
}

func TestDecodeHashFromHashOutput(t *testing.T) {
	encoded, err := Hash("test-password")
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	params, salt, hash, err := decodeHash(encoded)
	if err != nil {
		t.Fatalf("decodeHash returned error: %v", err)
	}

	if params.time != argonTime {
		t.Fatalf("unexpected time: got %d want %d", params.time, argonTime)
	}
	if params.memory != argonMemory {
		t.Fatalf("unexpected memory: got %d want %d", params.memory, argonMemory)
	}
	if params.threads != argonThreads {
		t.Fatalf("unexpected threads: got %d want %d", params.threads, argonThreads)
	}
	if len(salt) != saltLen {
		t.Fatalf("unexpected salt length: got %d want %d", len(salt), saltLen)
	}
	if len(hash) != argonKeyLen {
		t.Fatalf("unexpected hash length: got %d want %d", len(hash), argonKeyLen)
	}
}

func TestVerifyInvalidHashFormat(t *testing.T) {
	_, err := Verify("password", "not-a-valid-hash")
	if err == nil {
		t.Fatalf("expected error for invalid hash format")
	}
}

func TestDecodeHashWrongAlgorithm(t *testing.T) {
	encoded := "$bcrypt$v=1$t=3$m=65536$p=4$abcd$efgh"

	_, _, _, err := decodeHash(encoded)
	if err == nil {
		t.Fatalf("expected error for unsupported algorithm")
	}
}

func TestDecodeHashWrongVersion(t *testing.T) {
	encoded := "$argon2id$v=2$t=3$m=65536$p=4$abcd$efgh"

	_, _, _, err := decodeHash(encoded)
	if err == nil {
		t.Fatalf("expected error for unsupported version")
	}
}

func TestParseUintInvalidPrefix(t *testing.T) {
	_, err := parseUint("x=3", "t")
	if err == nil {
		t.Fatalf("expected error for invalid parameter prefix")
	}
}

func TestParseUintInvalidValue(t *testing.T) {
	_, err := parseUint("t=abc", "t")
	if err == nil {
		t.Fatalf("expected error for invalid parameter value")
	}
}
