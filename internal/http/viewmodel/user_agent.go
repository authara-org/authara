package viewmodel

import "github.com/authara-org/authara/internal/useragent"

func Label(ua string) string {
	return useragent.Parse(ua).Label()
}
