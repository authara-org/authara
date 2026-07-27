package response

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

func TestUserWithRolesEncodesEmptyRolesArray(t *testing.T) {
	user := UserWithRoles(domain.User{
		ID:       uuid.New(),
		Email:    "person@example.com",
		Username: "person",
	}, nil)

	body, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}
	if !json.Valid(body) {
		t.Fatalf("invalid JSON: %s", body)
	}
	if !strings.Contains(string(body), `"roles":[]`) {
		t.Fatalf("expected empty roles array, got %s", body)
	}
}
