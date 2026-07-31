package runtime

import (
	"encoding/base64"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/ompui/termcaps"
)

const (
	osc99MaxPayloadBytes = 2048
	osc99AppName         = "Oh My Pi"
)

// formatNotification builds the wire payload for n given the active protocol
// and whether OSC 99 structured support is confirmed.
func formatNotification(protocol termcaps.NotifyProtocol, osc99OK bool, n Notification, nextID *int) string {
	line := termcaps.FormatNotificationLine(n.Title, n.Body)
	switch {
	case protocol == termcaps.NotifyProtocolOSC99 && osc99OK:
		return formatOsc99Notification(n, nextID)
	case protocol == termcaps.NotifyProtocolOSC99:
		// Unconfirmed OSC 99: collapse to a single line via OSC 9-style notify.
		if line == "" {
			return string(termcaps.NotifyProtocolBell)
		}
		return termcaps.FormatNotification(termcaps.NotifyProtocolOSC9, line)
	case protocol == termcaps.NotifyProtocolBell || protocol == "":
		return string(termcaps.NotifyProtocolBell)
	default:
		if line == "" {
			return string(termcaps.NotifyProtocolBell)
		}
		return termcaps.FormatNotification(protocol, line)
	}
}

func formatOsc99Notification(n Notification, nextID *int) string {
	id := sanitizeOsc99ID(n.ID)
	if id == "" {
		*nextID++
		id = "omp-" + strconv.Itoa(*nextID)
	}
	meta := []string{"i=" + id, "f=" + base64.StdEncoding.EncodeToString([]byte(osc99AppName))}
	if a := osc99Actions(n.Actions); a != "" {
		meta = append(meta, "a="+a)
	}
	if u := osc99Urgency(n.Urgency); u != "" {
		meta = append(meta, "u="+u)
	}
	for _, t := range n.Type {
		if t == "" {
			continue
		}
		meta = append(meta, "t="+base64.StdEncoding.EncodeToString([]byte(t)))
	}
	if n.IconName != "" {
		meta = append(meta, "n="+base64.StdEncoding.EncodeToString([]byte(n.IconName)))
	}
	if n.Sound != "" {
		meta = append(meta, "s="+base64.StdEncoding.EncodeToString([]byte(n.Sound)))
	}
	if n.ExpiresMs != 0 {
		w := n.ExpiresMs
		if w < -1 {
			w = -1
		}
		meta = append(meta, "w="+strconv.Itoa(w))
	}

	title := n.Title
	if title == "" {
		title = n.Body
	}
	body := ""
	if n.Title != "" {
		body = n.Body
	}

	if body != "" {
		return osc99Payload(meta, title, true) + osc99Payload([]string{"i=" + id, "p=body"}, body, false)
	}
	return osc99Payload(meta, title, false)
}

func sanitizeOsc99ID(id string) string {
	if id == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '_' || r == '+' || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "0" {
		return ""
	}
	return out
}

func osc99Urgency(u Urgency) string {
	switch u {
	case UrgencyLow:
		return "0"
	case UrgencyNormal:
		return "1"
	case UrgencyCritical:
		return "2"
	default:
		return ""
	}
}

func osc99Actions(a NotifyActions) string {
	switch a {
	case NotifyActionsFocus:
		return "focus"
	case NotifyActionsReport:
		return "report"
	case NotifyActionsFocusReport:
		return "focus,report"
	case NotifyActionsNone:
		return "-focus"
	default:
		return ""
	}
}

func osc99Payload(meta []string, payload string, holdUntilLater bool) string {
	chunks := chunkUTF8(payload, osc99MaxPayloadBytes)
	var out strings.Builder
	for i, chunk := range chunks {
		m := append([]string(nil), meta...)
		if holdUntilLater || i < len(chunks)-1 {
			m = append(m, "d=0")
		}
		out.WriteString(osc99Chunk(m, chunk))
	}
	return out.String()
}

func osc99Chunk(meta []string, payload string) string {
	joined := strings.Join(meta, ":")
	if osc99Unsafe(payload) {
		return "\x1b]99;" + joined + ":e=1;" + base64.StdEncoding.EncodeToString([]byte(payload)) + "\x1b\\"
	}
	return "\x1b]99;" + joined + ";" + payload + "\x1b\\"
}

func osc99Unsafe(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == 0x7f || c >= 0x80 && c <= 0x9f {
			return true
		}
	}
	return false
}

func chunkUTF8(payload string, maxBytes int) []string {
	if payload == "" {
		return []string{""}
	}
	var chunks []string
	start := 0
	index := 0
	bytes := 0
	for index < len(payload) {
		_, size := utf8.DecodeRuneInString(payload[index:])
		if size <= 0 {
			size = 1
		}
		if bytes > 0 && bytes+size > maxBytes {
			chunks = append(chunks, payload[start:index])
			start = index
			bytes = 0
		}
		bytes += size
		index += size
	}
	chunks = append(chunks, payload[start:])
	return chunks
}

func parseOsc99KeyValues(section string) map[string]string {
	out := make(map[string]string)
	if section == "" {
		return out
	}
	for _, part := range strings.Split(section, ":") {
		eq := strings.IndexByte(part, '=')
		if eq <= 0 {
			continue
		}
		out[part[:eq]] = part[eq+1:]
	}
	return out
}
