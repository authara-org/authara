package viewmodel

import "strings"

func Label(ua string) string {
	ua = strings.ToLower(ua)

	browser := detectBrowser(ua)
	os := detectOS(ua)

	if browser == "" && os == "" {
		return "Unknown device"
	}
	if browser == "" {
		return os
	}
	if os == "" {
		return browser
	}
	return browser + " · " + os
}

func detectBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "edg/"):
		return "Edge"
	case strings.Contains(ua, "opr/") || strings.Contains(ua, "opera"):
		return "Opera"
	case strings.Contains(ua, "chrome/") && !strings.Contains(ua, "edg/"):
		return "Chrome"
	case strings.Contains(ua, "firefox/"):
		return "Firefox"
	case strings.Contains(ua, "safari/") && !strings.Contains(ua, "chrome/"):
		if strings.Contains(ua, "mobile") {
			return "Mobile Safari"
		}
		return "Safari"
	default:
		return ""
	}
}

func detectOS(ua string) string {
	switch {
	case strings.Contains(ua, "windows nt"):
		return "Windows"
	case strings.Contains(ua, "mac os x"):
		return "macOS"
	case strings.Contains(ua, "iphone"):
		return "iPhone"
	case strings.Contains(ua, "ipad"):
		return "iPad"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return ""
	}
}
