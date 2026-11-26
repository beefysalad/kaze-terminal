package tabs

// NextID returns the ID the next NewOpts call will use. It reserves nothing and
// is reliable only on the UI goroutine that serializes all tab creation.
func (m *Manager) NextID() TabID {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nextID + 1
}
