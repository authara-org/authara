package token

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/session/roles"
	"github.com/google/uuid"
)

func TestAccessTokenValidatesWithNodeStandardLibrary(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}

	keys, err := NewKeySet("cross-language", map[string][]byte{
		"cross-language": bytes.Repeat([]byte{7}, 32),
	})
	if err != nil {
		t.Fatal(err)
	}
	service := NewAccessTokenService(keys, "authara-cross-language", 10*time.Minute)
	raw, err := service.Generate(
		uuid.New(), uuid.New(), uuid.New(), "owner", AudienceApp, roles.Roles{}, time.Now().UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}

	input, err := json.Marshal(map[string]any{
		"token":    raw,
		"jwks":     keys.JWKS(),
		"issuer":   "authara-cross-language",
		"audience": "app",
	})
	if err != nil {
		t.Fatal(err)
	}

	command := exec.Command(node, filepath.Join("testdata", "verify_jwt.mjs"))
	command.Stdin = bytes.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Node verification failed: %v\n%s", err, output)
	}
	if string(output) != "ok\n" {
		t.Fatalf("unexpected Node output %q", output)
	}
}
