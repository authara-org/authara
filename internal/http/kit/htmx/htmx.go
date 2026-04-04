package htmx

import "net/http"

const (
	HTMXPushUrl    = "HX-Push-Url"
	HTMXReplaceUrl = "HX-Replace-Url"
	HTMXRetarget   = "HX-Retarget"
	HTMXReswap     = "HX-Reswap"
)

func PushUrl(w http.ResponseWriter, url string) {
	w.Header().Add(HTMXPushUrl, url)
}

func ReplaceUrl(w http.ResponseWriter, url string) {
	w.Header().Add(HTMXReplaceUrl, url)
}

func ReTarget(w http.ResponseWriter, target string) {
	w.Header().Add(HTMXRetarget, target)
}

func ReSwap(w http.ResponseWriter, swap string) {
	w.Header().Add(HTMXReswap, swap)
}
