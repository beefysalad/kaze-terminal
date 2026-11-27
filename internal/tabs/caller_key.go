package tabs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

func (m *Manager) deriveOwnerKey(id TabID) string {
	mac := hmac.New(sha256.New, m.callerSecret[:])
	mac.Write([]byte("t" + strconv.Itoa(int(id))))
	return hex.EncodeToString(mac.Sum(nil))
}

// OwnerKey returns a key only for an open, owner-created tab. It changes no tab
// state and is safe from any goroutine. Keys must never enter public snapshots.
func (m *Manager) OwnerKey(id TabID) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := m.indexLocked(id)
	if i < 0 || m.entries[i].tab.Parent != 0 || m.entries[i].tab.Role != "" {
		return ""
	}
	return m.deriveOwnerKey(id)
}
