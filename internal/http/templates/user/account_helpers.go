package user

func displayName(username, email string) string {
	if username != "" {
		return username
	}
	return email
}

func initials(username, email string) string {
	s := username
	if s == "" {
		s = email
	}

	for i, _ := range s {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			if c >= 'a' && c <= 'z' {
				c = c - 'a' + 'A'
			}
			return string([]byte{c})
		}
	}

	return "U"
}
