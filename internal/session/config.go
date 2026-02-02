package session

var secureCookies = true

func Configure(secure bool) {
	secureCookies = secure
}
