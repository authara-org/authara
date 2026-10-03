package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestTrustedRealIPIgnoresHeadersFromUntrustedPeer(t *testing.T) {
	remoteAddr := captureRemoteAddr(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, "198.51.100.9:1234", http.Header{
		"X-Forwarded-For": []string{"203.0.113.10"},
		"X-Real-Ip":       []string{"203.0.113.11"},
		"True-Client-Ip":  []string{"203.0.113.12"},
	})

	if remoteAddr != "198.51.100.9:1234" {
		t.Fatalf("RemoteAddr = %q, want transport peer unchanged", remoteAddr)
	}
}

func TestTrustedRealIPResolvesForwardedChainFromRightToLeft(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.0.2.0/24"),
	}
	remoteAddr := captureRemoteAddr(t, trusted, "10.0.0.5:443", http.Header{
		"X-Forwarded-For": []string{"203.0.113.200, 198.51.100.25", "192.0.2.40"},
	})

	if remoteAddr != "198.51.100.25" {
		t.Fatalf("RemoteAddr = %q, want first untrusted hop", remoteAddr)
	}
}

func TestTrustedRealIPUsesXRealIPFromTrustedPeer(t *testing.T) {
	remoteAddr := captureRemoteAddr(t, []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/48")}, "[2001:db8:1::5]:443", http.Header{
		"X-Real-Ip": []string{"2001:db8:2::20"},
	})

	if remoteAddr != "2001:db8:2::20" {
		t.Fatalf("RemoteAddr = %q, want forwarded IPv6 client", remoteAddr)
	}
}

func TestTrustedRealIPRejectsMalformedForwardedForAndTrueClientIP(t *testing.T) {
	remoteAddr := captureRemoteAddr(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, "10.0.0.5:443", http.Header{
		"X-Forwarded-For": []string{"203.0.113.20, not-an-ip"},
		"True-Client-Ip":  []string{"203.0.113.30"},
	})

	if remoteAddr != "10.0.0.5:443" {
		t.Fatalf("RemoteAddr = %q, want transport peer unchanged", remoteAddr)
	}
}

func TestTrustedRealIPFallsBackToXRealIPWhenForwardedForIsMalformed(t *testing.T) {
	remoteAddr := captureRemoteAddr(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, "10.0.0.5:443", http.Header{
		"X-Forwarded-For": []string{"203.0.113.20, not-an-ip"},
		"X-Real-Ip":       []string{"198.51.100.25"},
	})

	if remoteAddr != "198.51.100.25" {
		t.Fatalf("RemoteAddr = %q, want trusted proxy's X-Real-IP fallback", remoteAddr)
	}
}

func TestTrustedRealIPUnmapsIPv4MappedPeer(t *testing.T) {
	remoteAddr := captureRemoteAddr(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, "[::ffff:10.0.0.5]:443", http.Header{
		"X-Forwarded-For": []string{"203.0.113.20"},
	})

	if remoteAddr != "203.0.113.20" {
		t.Fatalf("RemoteAddr = %q, want forwarded client", remoteAddr)
	}
}

func captureRemoteAddr(t *testing.T, trusted []netip.Prefix, peer string, headers http.Header) string {
	t.Helper()
	var remoteAddr string
	handler := TrustedRealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		remoteAddr = r.RemoteAddr
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = peer
	req.Header = headers
	handler.ServeHTTP(httptest.NewRecorder(), req)
	return remoteAddr
}
