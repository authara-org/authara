package htmx

import "net/http"

const (
	HTMXPushUrl    = "HX-Push-Url"
	HTMXReplaceUrl = "HX-Replace-Url"
)

func PushUrl(w http.ResponseWriter, url string) {
	w.Header().Add(HTMXPushUrl, url)
}

func ReplaceUrl(w http.ResponseWriter, url string) {
	w.Header().Add(HTMXReplaceUrl, url)
}

