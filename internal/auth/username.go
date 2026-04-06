package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

func ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 30 {
		return ErrInvalidUsername
	}

	for i := 0; i < len(username); i++ {
		c := username[i]

		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' ||
			c == '_' {
			continue
		}

		return ErrInvalidUsername
	}

	return nil
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

var maxFiveDigits = big.NewInt(90000)

func SecureFiveDigits() (int64, error) {
	n, err := rand.Int(rand.Reader, maxFiveDigits)
	if err != nil {
		return 0, err
	}
	return n.Int64() + 10000, nil
}

// sanitizeUsername:
// - keeps only [a-zA-z0-9_-]
// - turns runs of other chars into a single '-'
// - trims leading/trailing '-' and '_'
func SanitizeUsername(s string) string {
	const maxLen = 24

	var b strings.Builder
	b.Grow(len(s))

	prevDash := false
	for _, r := range s {
		// keep ASCII letters/digits/_/-
		if isLetter(r) || isDigit(r) || r == '_' {
			b.WriteRune(r)
			prevDash = false
			continue
		}

		// treat any whitespace/punct/etc as separator => single '-'
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			if b.Len() > 0 && !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
			continue
		}

		// everything else (e.g. non-ascii letters) => separator too
		if b.Len() > 0 && !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}

	out := b.String()
	out = strings.Trim(out, "-_")
	if len(out) > maxLen {
		out = strings.Trim(out[:maxLen], "-_")
	}
	return out
}

func EnsureUsername(username, email string) (string, error) {
	if username != "" {
		return username, nil
	}

	local := strings.SplitN(email, "@", 2)[0]
	local = SanitizeUsername(local)

	if local == "" {
		local = "user"
	}

	suffix, err := SecureFiveDigits()
	if err != nil {
		return "", err
	}

	local = strings.ToLower(local)

	return fmt.Sprintf("%s-%05d", local, suffix), nil
}
