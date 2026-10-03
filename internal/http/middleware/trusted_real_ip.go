package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// TrustedRealIP resolves a client address from forwarding headers only when
// the request's transport peer belongs to a configured trusted proxy network.
func TrustedRealIP(trustedProxies []netip.Prefix) func(http.Handler) http.Handler {
	trustedProxies = append([]netip.Prefix(nil), trustedProxies...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, ok := parseRemoteIP(r.RemoteAddr)
			if ok && addressInPrefixes(peer, trustedProxies) {
				if client, ok := forwardedClientIP(r, trustedProxies); ok {
					r.RemoteAddr = client.String()
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func forwardedClientIP(r *http.Request, trustedProxies []netip.Prefix) (netip.Addr, bool) {
	if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
		chain, ok := parseForwardedFor(values)
		if ok {
			for i := len(chain) - 1; i >= 0; i-- {
				if !addressInPrefixes(chain[i], trustedProxies) {
					return chain[i], true
				}
			}
			if len(chain) > 0 {
				return chain[0], true
			}
		}
	}

	values := r.Header.Values("X-Real-IP")
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return netip.Addr{}, false
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(values[0]))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func parseForwardedFor(values []string) ([]netip.Addr, bool) {
	var chain []netip.Addr
	for _, value := range values {
		for _, raw := range strings.Split(value, ",") {
			ip, err := netip.ParseAddr(strings.TrimSpace(raw))
			if err != nil {
				return nil, false
			}
			chain = append(chain, ip.Unmap())
		}
	}
	return chain, len(chain) > 0
}

func parseRemoteIP(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		remoteAddr = host
	}
	ip, err := netip.ParseAddr(strings.Trim(remoteAddr, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func addressInPrefixes(ip netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}
