package ui

import (
	"strings"
	"testing"
)

func TestTourStepsOrder(t *testing.T) {
	ss := tourSteps()
	if len(ss) != 6 || !ss[5].Hooks || !ss[4].Board {
		t.Fatal(ss)
	}
	for _, s := range ss {
		for _, bad := range []string{"session", "orchestrat", "message", "pane", "—"} {
			if strings.Contains(strings.ToLower(s.Body), bad) {
				t.Fatal(bad)
			}
		}
	}
}
