package termcaps

import "strings"

// IsNotificationSuppressed reports whether PI_NOTIFICATIONS disables notifications.
// Values "off", "0", and "false" suppress; unset or any other value does not.
func IsNotificationSuppressed(env Env) bool {
	v := env.Get("PI_NOTIFICATIONS")
	if v == "" {
		return false
	}
	return v == "off" || v == "0" || v == "false"
}

// FormatNotification builds a notification escape sequence for protocol.
// Structured title/body collapse is left to callers with richer types; this
// helper formats a single message line (or bare bell).
func FormatNotification(protocol NotifyProtocol, message string) string {
	if protocol == NotifyProtocolBell || protocol == "" {
		return string(NotifyProtocolBell)
	}
	return string(protocol) + message + "\x1b\\"
}

// FormatNotificationLine collapses title and body into one "title: body" line
// for non-OSC-99 sinks (and unconfirmed OSC 99).
func FormatNotificationLine(title, body string) string {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	switch {
	case title == "" && body == "":
		return ""
	case title == "":
		return body
	case body == "":
		return title
	default:
		return title + ": " + body
	}
}
