package user

import "net/url"

func accountPageHref(sessionsCursor, passkeysCursor string) string {
	values := url.Values{}
	if sessionsCursor != "" {
		values.Set("sessions_cursor", sessionsCursor)
	}
	if passkeysCursor != "" {
		values.Set("passkeys_cursor", passkeysCursor)
	}
	return "/auth/account?" + values.Encode()
}
