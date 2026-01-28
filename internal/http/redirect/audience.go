package redirect

import (
	"strings"

	"github.com/alexlup06/authgate/internal/session/token"
)

func AudienceForPath(path string) token.Audience {
	if strings.HasPrefix(path, "/admin") {
		return token.AudienceAdmin
	}
	return token.AudienceApp
}
