package tabs_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestCallerKeyEnvironment(t *testing.T) {
	t.Setenv("KAZE_TAB_KEY", "inherited")
	m, st := newManager(t)
	defer m.CloseAll()
	opts := []tabs.StartOpts{
		{}, {},
		{Parent: 1, Role: "worker", Env: []string{"KAZE_TAB_KEY=forged", "kaze_tab_key=also-forged"}},
		{Parent: 1}, {Role: "worker"},
		{Env: []string{"KAZE_TAB_KEY=forged"}},
	}
	keys := make([]string, len(opts))
	for i, o := range opts {
		if _, err := m.NewOpts(o); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, e := range st.envs[i] {
			name, value, _ := strings.Cut(e, "=")
			if strings.EqualFold(name, "KAZE_TAB_KEY") {
				count++
				keys[i] = value
			}
		}
		if o.Parent != 0 || o.Role != "" {
			if count != 0 {
				t.Fatalf("worker %d received caller key", i)
			}
		} else {
			b, err := hex.DecodeString(keys[i])
			if count != 1 || err != nil || len(b) != 32 {
				t.Fatalf("owner %d missing unique HMAC key", i)
			}
		}
	}
	if keys[0] == keys[1] {
		t.Fatal("tab keys repeated")
	}
	other, otherST := newManager(t)
	defer other.CloseAll()
	mustNew(t, other)
	for _, e := range otherST.envs[0] {
		if e == "KAZE_TAB_KEY="+keys[0] {
			t.Fatal("window keys repeated")
		}
	}
}
