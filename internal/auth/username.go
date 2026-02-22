package auth

func ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 30 {
		return ErrInvalidUsername
	}

	for i := 0; i < len(username); i++ {
		c := username[i]

		if isLetter(c) || isDigit(c) || c == '-' || c == '_' {
			continue
		}

		return ErrInvalidUsername
	}

	return nil
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
