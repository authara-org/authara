package useragent

import "strings"

type DeviceKind string

const (
	DeviceDesktop DeviceKind = "desktop"
	DevicePhone   DeviceKind = "phone"
	DeviceTablet  DeviceKind = "tablet"
	DeviceUnknown DeviceKind = "unknown"
)

type Agent struct {
	Raw        string
	Browser    string
	OS         string
	Device     string
	DeviceKind DeviceKind
	Mobile     bool
}

func Parse(raw string) Agent {
	ua := normalize(raw)

	return Agent{
		Raw:        raw,
		Browser:    detectBrowser(ua),
		OS:         detectOS(ua),
		Device:     detectDevice(ua),
		DeviceKind: detectDeviceKind(ua),
		Mobile:     strings.Contains(ua, "mobile"),
	}
}

func (a Agent) BrowserSummary() string {
	if strings.TrimSpace(a.Raw) == "" {
		return "Unknown browser"
	}
	if a.Browser == "" {
		return "Unknown browser"
	}
	if a.OS == "" {
		return a.Browser
	}
	return a.Browser + " on " + a.OS
}

func (a Agent) Label() string {
	browser := a.Browser
	if browser == "Safari" && a.Mobile {
		browser = "Mobile Safari"
	}

	platform := a.Device
	if platform == "" {
		platform = a.OS
	}

	if browser == "" && platform == "" {
		return "Unknown device"
	}
	if browser == "" {
		return platform
	}
	if platform == "" {
		return browser
	}
	return browser + " · " + platform
}

type PlatformSignals struct {
	hint  string
	agent Agent
}

func NewPlatformSignals(platformHint, rawUserAgent string) PlatformSignals {
	return PlatformSignals{
		hint:  normalize(platformHint),
		agent: Parse(rawUserAgent),
	}
}

func (s PlatformSignals) IsIPhone() bool {
	return contains(s.hint, "iphone") || s.agent.Device == "iPhone"
}

func (s PlatformSignals) IsIPad() bool {
	return contains(s.hint, "ipad") || s.agent.Device == "iPad"
}

func (s PlatformSignals) IsIOS() bool {
	return s.hint == "ios" || s.IsIPhone() || s.IsIPad() || s.agent.OS == "iOS"
}

func (s PlatformSignals) IsMacOS() bool {
	return containsAny(s.hint, "macos", "mac os", "mac") || s.agent.OS == "macOS"
}

func (s PlatformSignals) IsChromeOS() bool {
	return containsAny(s.hint, "chrome os", "chromium os", "cros") || s.agent.OS == "ChromeOS"
}

func (s PlatformSignals) IsWindows() bool {
	return contains(s.hint, "windows") || s.agent.OS == "Windows"
}

func (s PlatformSignals) IsAndroid() bool {
	return contains(s.hint, "android") || s.agent.OS == "Android"
}

func (s PlatformSignals) IsLinux() bool {
	return contains(s.hint, "linux") || s.agent.OS == "Linux"
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func detectBrowser(ua string) string {
	switch {
	case contains(ua, "edg/"):
		return "Edge"
	case containsAny(ua, "opr/", "opera"):
		return "Opera"
	case containsAny(ua, "crios/", "chrome/", "chromium/"):
		return "Chrome"
	case containsAny(ua, "fxios/", "firefox/"):
		return "Firefox"
	case contains(ua, "safari/"):
		return "Safari"
	default:
		return ""
	}
}

func detectOS(ua string) string {
	switch {
	case containsAny(ua, "iphone", "ipad"):
		return "iOS"
	case containsAny(ua, "mac os x", "macintosh"):
		return "macOS"
	case contains(ua, "windows"):
		return "Windows"
	case contains(ua, "android"):
		return "Android"
	case contains(ua, "cros"):
		return "ChromeOS"
	case contains(ua, "linux"):
		return "Linux"
	default:
		return ""
	}
}

func detectDevice(ua string) string {
	switch {
	case contains(ua, "iphone"):
		return "iPhone"
	case contains(ua, "ipad"):
		return "iPad"
	default:
		return ""
	}
}

func detectDeviceKind(ua string) DeviceKind {
	switch {
	case ua == "":
		return DeviceUnknown
	case containsAny(ua, "ipad", "tablet"):
		return DeviceTablet
	case contains(ua, "iphone"), contains(ua, "android") && contains(ua, "mobile"):
		return DevicePhone
	default:
		return DeviceDesktop
	}
}

func contains(value, needle string) bool {
	return strings.Contains(value, needle)
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if contains(value, needle) {
			return true
		}
	}
	return false
}
