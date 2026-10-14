package csrf

var secureCookies = true
var cookieDomain string

func Configure(secure bool, domain string) {
	secureCookies = secure
	cookieDomain = domain
}
