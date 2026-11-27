package tabs

import "testing"

func TestNextID(t *testing.T) {
	m := &Manager{nextID: 7}
	if got := m.NextID(); got != 8 {
		t.Fatal(got)
	}
	if got := m.NextID(); got != 8 {
		t.Fatal("read reserved an ID", got)
	}
}
