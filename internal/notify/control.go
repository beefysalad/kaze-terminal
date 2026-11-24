package notify

import (
	"encoding/json"
	"net"
	"time"
)

func controlError(code, message string) any {
	return map[string]any{"ok": false, "error": map[string]string{"code": code, "message": message}}
}

func (s *Server) writeControl(c net.Conn, reply any) {
	c.SetWriteDeadline(time.Now().Add(s.deadline))
	json.NewEncoder(c).Encode(reply)
}
