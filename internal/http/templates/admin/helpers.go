package admin

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	adminsvc "github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/http/templates/components/button"
	"github.com/google/uuid"
)

type kv struct {
	Key   string
	Value string
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	return t.Format("2006-01-02 15:04 MST")
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "Never"
	}
	return formatTime(*t)
}

func stringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func uuidPtrString(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func uuidPtrShort(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return shortID(*value)
}

func shortID(id uuid.UUID) string {
	text := id.String()
	if len(text) <= 8 {
		return text
	}
	return text[:8]
}

func maskedStringPtr(value *string) string {
	if value == nil || *value == "" {
		return ""
	}
	return maskEmail(*value)
}

func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "hidden"
	}
	local := parts[0]
	if local == "" {
		return "***@" + parts[1]
	}
	return local[:1] + "***@" + parts[1]
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return "None"
	}
	return strings.Join(values, ", ")
}

func navButtonColor(active bool) button.Color {
	if active {
		return button.AdminNavActive
	}
	return button.AdminNav
}

func badgeClass(color string) string {
	switch color {
	case "green":
		return "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-200"
	case "red":
		return "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-200"
	case "blue":
		return "bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-200"
	default:
		return "bg-grey-100 text-grey-700 dark:bg-grey-800 dark:text-grey-200"
	}
}

func pageHref(base string, query string, page int) string {
	if query == "" {
		return fmt.Sprintf("%s?page=%d", base, page)
	}
	return fmt.Sprintf("%s?q=%s&page=%d", base, query, page)
}

func allowlistResultsHref(query string, page int) string {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	if page > 1 {
		values.Set("page", fmt.Sprint(page))
	}
	if encoded := values.Encode(); encoded != "" {
		return "/auth/admin/allowlist/results?" + encoded
	}
	return "/auth/admin/allowlist/results"
}

func allowlistMutationHref(base string, query string, page int) string {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	if page > 1 {
		values.Set("page", fmt.Sprint(page))
	}
	if encoded := values.Encode(); encoded != "" {
		return base + "?" + encoded
	}
	return base
}

func allowlistShowingText(page adminsvc.AllowedEmailPage) string {
	if page.Total == 0 {
		return "Showing 0 emails"
	}
	start := (page.Page-1)*page.Size + 1
	end := page.Page * page.Size
	if end > page.Total {
		end = page.Total
	}
	return fmt.Sprintf("Showing %d-%d of %d emails", start, end, page.Total)
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return string(raw)
	}
	out = redactJSONMap(out)
	if len(out) == 0 {
		return "{}"
	}
	buf, err := json.Marshal(out)
	if err != nil {
		return string(raw)
	}
	return string(buf)
}

func redactJSONMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		if sensitiveJSONKey(key) {
			out[key] = "[redacted]"
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			out[key] = redactJSONMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}

func sensitiveJSONKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, token := range []string{
		"password",
		"token",
		"hash",
		"secret",
		"code",
		"credential",
		"public_key",
		"request_body",
		"body",
	} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}
