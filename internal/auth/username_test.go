package auth

import "testing"

func TestValidateUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{name: "min length", username: "abc", wantErr: false},
		{name: "max length", username: "abcdefghijklmnopqrstuvwxyz1234", wantErr: false}, // 30 chars
		{name: "too short", username: "ab", wantErr: true},
		{name: "too long", username: "abcdefghijklmnopqrstuvwxyz12345", wantErr: true}, // 31 chars
		{name: "letters numbers dash underscore", username: "user_Name-123", wantErr: false},
		{name: "contains space", username: "user name", wantErr: true},
		{name: "contains dot", username: "user.name", wantErr: true},
		{name: "contains unicode", username: "üser", wantErr: true},
		{name: "empty", username: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateUsername(tt.username)
			gotErr := err != nil

			if gotErr != tt.wantErr {
				t.Fatalf("ValidateUsername(%q) error = %v, wantErr %v", tt.username, err, tt.wantErr)
			}

			if tt.wantErr && err != ErrInvalidUsername {
				t.Fatalf("ValidateUsername(%q) error = %v, want %v", tt.username, err, ErrInvalidUsername)
			}
		})
	}
}

func TestSecureFiveDigits(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		n, err := SecureFiveDigits()
		if err != nil {
			t.Fatalf("SecureFiveDigits() error = %v", err)
		}
		if n < 10000 || n > 99999 {
			t.Fatalf("SecureFiveDigits() = %d, want value in range [10000, 99999]", n)
		}
	}
}

func TestSanitizeUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "keeps allowed chars", in: "user_Name-123", want: "user_Name-123"},
		{name: "spaces become single dash", in: "john doe", want: "john-doe"},
		{name: "multiple spaces collapse", in: "john   doe", want: "john-doe"},
		{name: "punctuation collapses", in: "john@doe.com", want: "john-doe-com"},
		{name: "leading separators trimmed", in: "___john", want: "john"},
		{name: "trailing separators trimmed", in: "john---", want: "john"},
		{name: "leading and trailing separators trimmed", in: "__john-doe__", want: "john-doe"},
		{name: "only punctuation becomes empty", in: "...---___", want: ""},
		{name: "whitespace only becomes empty", in: "   \t\n   ", want: ""},
		{name: "non ascii letters become separator", in: "älex müller", want: "lex-m-ller"},
		{name: "mixed punctuation and spaces collapse to single dash", in: "john -.- doe", want: "john-doe"},
		{name: "starts with punctuation", in: "...john", want: "john"},
		{name: "ends with punctuation", in: "john...", want: "john"},
		{name: "empty", in: "", want: ""},
		{name: "max len truncates and trims", in: "abcdefghijklmnopqrstuvwxyz", want: "abcdefghijklmnopqrstuvwx"},
		{name: "truncate then trim separators", in: "abcdefghijklmnopqrstuvwx---yz", want: "abcdefghijklmnopqrstuvwx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SanitizeUsername(tt.in)
			if got != tt.want {
				t.Fatalf("SanitizeUsername(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEnsureUsername_ReturnsExistingUsername(t *testing.T) {
	got, err := EnsureUsername("already-set", "user@example.com")
	if err != nil {
		t.Fatalf("EnsureUsername returned error: %v", err)
	}
	if got != "already-set" {
		t.Fatalf("expected already-set, got %q", got)
	}
}

func TestEnsureUsername_GeneratesFromEmail(t *testing.T) {
	got, err := EnsureUsername("", "john.doe@example.com")
	if err != nil {
		t.Fatalf("EnsureUsername returned error: %v", err)
	}
	if got == "" {
		t.Fatal("expected generated username, got empty string")
	}
}

func TestEnsureUsername_FallsBackToUser(t *testing.T) {
	got, err := EnsureUsername("", "@example.com")
	if err != nil {
		t.Fatalf("EnsureUsername returned error: %v", err)
	}
	if len(got) < len("user-10000") {
		t.Fatalf("expected fallback username, got %q", got)
	}
}
