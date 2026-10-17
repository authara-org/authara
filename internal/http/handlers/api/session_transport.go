package api

import "net/http"

const tokenSessionTransport = "token"

func tokenSessionRequested(r *http.Request) bool {
	return r != nil && r.URL.Query().Get("session_transport") == tokenSessionTransport
}
