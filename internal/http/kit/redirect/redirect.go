package redirect

import "net/http"

func Redirect(w http.ResponseWriter, r *http.Request, location string, status int) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("HX-Redirect", location)
	http.Redirect(w, r, location, status)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
