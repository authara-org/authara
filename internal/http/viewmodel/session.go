package viewmodel

import (
	"fmt"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type DeviceKind string

const (
	DeviceDesktop DeviceKind = "desktop"
	DevicePhone   DeviceKind = "phone"
	DeviceTablet  DeviceKind = "tablet"
	DeviceUnknown DeviceKind = "unknown"
)

type Session struct {
	ID         uuid.UUID
	IsCurrent  bool
	Title      string
	Subtitle   string
	DeviceKind DeviceKind
}

func SessionFromDomain(s domain.Session, currentSessionID uuid.UUID) Session {
	label := Label(s.UserAgent)

	return Session{
		ID:         s.ID,
		IsCurrent:  s.ID == currentSessionID,
		Title:      sessionTitle(s, currentSessionID, label),
		Subtitle:   sessionSubtitle(s, label),
		DeviceKind: DeviceKindFromUserAgent(s.UserAgent),
	}
}

func DeviceKindFromUserAgent(ua string) DeviceKind {
	ua = strings.ToLower(ua)

	switch {
	case strings.Contains(ua, "ipad"), strings.Contains(ua, "tablet"):
		return DeviceTablet
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "android") && strings.Contains(ua, "mobile"):
		return DevicePhone
	case ua == "":
		return DeviceUnknown
	default:
		return DeviceDesktop
	}
}

func sessionTitle(session domain.Session, currentSessionID uuid.UUID, label string) string {
	if session.ID == currentSessionID {
		return "Current session"
	}
	if label != "" && label != "Unknown device" {
		return label
	}
	return "Other session"
}

func sessionSubtitle(session domain.Session, label string) string {
	if label == "" || label == "Unknown device" {
		return "Created " + formatSessionTime(session.CreatedAt)
	}
	return fmt.Sprintf("%s · Created %s", label, formatSessionTime(session.CreatedAt))
}

func formatSessionTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 UTC")
}
