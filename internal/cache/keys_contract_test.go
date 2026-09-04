package cache

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type revocationKeyContract struct {
	Token      string `json:"token"`
	Session    string `json:"session"`
	User       string `json:"user"`
	Membership string `json:"membership"`
}

func TestAccessTokenRevocationKeysMatchContract(t *testing.T) {
	data, err := os.ReadFile("../../contract/access-token-revocations.json")
	if err != nil {
		t.Fatal(err)
	}
	var templates revocationKeyContract
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&templates); err != nil {
		t.Fatal(err)
	}

	replacer := strings.NewReplacer(
		"{token_sha256}", "token-hash",
		"{session_id}", "session-id",
		"{user_id}", "user-id",
		"{organization_id}", "organization-id",
	)
	want := revocationKeyContract{
		Token:      replacer.Replace(templates.Token),
		Session:    replacer.Replace(templates.Session),
		User:       replacer.Replace(templates.User),
		Membership: replacer.Replace(templates.Membership),
	}
	got := revocationKeyContract{
		Token:      RevokedAccessTokenKey("token-hash"),
		Session:    RevokedAccessTokenSessionKey("session-id"),
		User:       RevokedAccessTokenUserKey("user-id"),
		Membership: RevokedAccessTokenMembershipKey("user-id", "organization-id"),
	}
	if got != want {
		t.Fatalf("revocation keys differ from contract:\n got: %+v\nwant: %+v", got, want)
	}
}
