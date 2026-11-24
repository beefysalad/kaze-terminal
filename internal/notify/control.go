package notify

import (
	"encoding/json"
	"net"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxControlRequest = 64 << 10
	maxControlReply   = 1 << 20 // includes the terminating newline
)

func controlError(code, message string) any {
	return map[string]any{"ok": false, "error": map[string]string{"code": code, "message": message}}
}

func (s *Server) writeControl(c net.Conn, reply any) {
	s.writeControlReply(c, reply, false)
}

func (s *Server) writeControlReply(c net.Conn, reply any, newest bool) {
	b, err := json.Marshal(reply)
	if err != nil {
		b, _ = json.Marshal(controlError("invalid_reply", "control reply could not be encoded"))
	}
	if len(b)+1 > maxControlReply {
		b = truncateControlReply(b, newest)
	}
	c.SetWriteDeadline(time.Now().Add(s.deadline))
	c.Write(append(b, '\n'))
}

// Preserve the newest complete lines for reads, and a text prefix for other ops. Large structured results have no safe
// generic partial representation, so return an explicit truncated error instead.
// The wire cap includes JSON escaping, metadata and the terminating newline.
func truncateControlReply(b []byte, newest bool) []byte {
	var fields map[string]json.RawMessage
	var text string
	if json.Unmarshal(b, &fields) == nil && json.Unmarshal(fields["text"], &text) == nil {
		fields["truncated"] = json.RawMessage("true")
		fields["text"] = json.RawMessage(`""`)
		metadata, _ := json.Marshal(fields)
		textBudget := maxControlReply - 1 - (len(metadata) - 2)
		part := func(n int) string {
			if !newest {
				return cutBytes(text, n)
			}
			start := len(text) - n
			for start < len(text) && !utf8.RuneStart(text[start]) {
				start++
			}
			return text[start:]
		}
		lo, hi := 0, len(text)
		for lo < hi {
			mid := lo + (hi-lo+1)/2
			prefix, _ := json.Marshal(part(mid))
			if len(prefix) <= textBudget {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		kept := part(lo)
		if newest {
			start := len(text) - len(kept)
			if start > 0 && text[start-1] != '\n' {
				// Drop the first partial line, even if it is the only line.
				if i := strings.IndexByte(kept, '\n'); i >= 0 {
					kept = kept[i+1:]
				} else {
					kept = ""
				}
			}
		}
		fields["text"], _ = json.Marshal(kept)
		candidate, _ := json.Marshal(fields)
		if len(candidate)+1 <= maxControlReply {
			return candidate
		}
	}
	fallback := controlError("reply_too_large", "control reply exceeds 1 MiB").(map[string]any)
	fallback["truncated"] = true
	b, _ = json.Marshal(fallback)
	return b
}
