package httputil

import (
	"net"
	"net/http"
)

func ClientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(r.RemoteAddr)
}

func ClientIPString(r *http.Request) string {
	ip := ClientIP(r)
	if ip == nil {
		return ""
	}
	return ip.String()
}
