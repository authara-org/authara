package useragent

import "testing"

func TestBrowserSummary(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
	}{
		{
			name: "chrome on macos",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
			want: "Chrome on macOS",
		},
		{
			name: "empty",
			ua:   " ",
			want: "Unknown browser",
		},
		{
			name: "unknown browser with os",
			ua:   "Mozilla/5.0 (X11; Linux x86_64)",
			want: "Unknown browser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.ua).BrowserSummary(); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestLabel(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
	}{
		{
			name: "chrome macos",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
			want: "Chrome · macOS",
		},
		{
			name: "mobile safari iphone",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
			want: "Mobile Safari · iPhone",
		},
		{
			name: "unknown",
			ua:   "",
			want: "Unknown device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.ua).Label(); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestDeviceKind(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want DeviceKind
	}{
		{name: "empty", want: DeviceUnknown},
		{name: "iphone", ua: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)", want: DevicePhone},
		{name: "ipad", ua: "Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X)", want: DeviceTablet},
		{name: "desktop", ua: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", want: DeviceDesktop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.ua).DeviceKind; got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestPlatformSignals(t *testing.T) {
	signals := NewPlatformSignals("macOS", "")
	if !signals.IsMacOS() {
		t.Fatal("expected macOS platform hint to be detected")
	}

	signals = NewPlatformSignals("", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)")
	if !signals.IsIOS() || !signals.IsIPhone() {
		t.Fatal("expected iPhone user agent to be detected")
	}
}
