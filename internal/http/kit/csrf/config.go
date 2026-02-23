package csrf

var secureCookies = true

func Configure(secure bool) {
	secureCookies = secure
}
