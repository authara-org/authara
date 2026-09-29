package identity

import "testing"

func TestCanonicalIdentity(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) string
		in   string
		want string
	}{
		{name: "email", fn: CanonicalEmail, in: "  Alice@Example.COM  ", want: "alice@example.com"},
		{name: "username", fn: CanonicalUsername, in: "  Alice_01  ", want: "alice_01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.in); got != tt.want {
				t.Fatalf("canonical value = %q, want %q", got, tt.want)
			}
		})
	}
}
