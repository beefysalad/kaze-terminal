package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestDocMatchWholeComponents(t *testing.T) {
	for _, c := range []struct {
		rec, ref string
		want     bool
	}{
		{"/v/tickets/V3-1/plan.md", "tickets/V3-1/plan.md", true},
		{"/v/tickets/V3-1/plan.md", "plan.md", true},
		{"/v/xtickets/a.md", "tickets/a.md", false},
		{"/v/tickets/V3-1/plan.md", "../plan.md", false},
		{"/v/tickets/V3-1/plan.md", "a/../plan.md", false},
		{"/v/tickets/V3-1/plan.md", "", false},
		{"/v/tickets/V3-1/plan.md", "./tickets/V3-1/plan.md", true},
		{"/v/plan.md", "x/y/v/plan.md", false},
	} {
		if got := docTailMatch(c.rec, c.ref, "linux"); got != c.want {
			t.Errorf("docTailMatch(%q, %q) = %v, want %v", c.rec, c.ref, got, c.want)
		}
	}
}

func TestDocMatchCaseAndSlashes(t *testing.T) {
	rec := filepath.Join("C:", "v", "Tickets", "v3-1", "Plan.md")
	if string(filepath.Separator) == `\` {
		rec = `C:\v\Tickets\v3-1\Plan.md`
	}
	for goos, want := range map[string]bool{"windows": true, "darwin": true, "linux": false} {
		if got := docTailMatch(rec, "tickets/V3-1/plan.md", goos); got != want {
			t.Errorf("%s: %v, want %v", goos, got, want)
		}
	}
}

func TestDocSeenNewestWins(t *testing.T) {
	dir := t.TempDir()
	a := writeDoc(t, dir, "A/plan.md", "a")
	b := writeDoc(t, dir, "B/plan.md", "b")
	s := &docSeen{}
	s.add(1, []string{a})
	s.add(1, []string{b})
	if got := s.find(1, "plan.md"); got != b {
		t.Fatalf("got %q, want %q", got, b)
	}
	s.add(1, []string{a})
	if got := s.find(1, "plan.md"); got != a {
		t.Fatalf("got %q, want %q", got, a)
	}
}

func TestDocSeenCapAndDedup(t *testing.T) {
	s := &docSeen{goos: "linux"}
	for i := range 70 {
		s.add(1, []string{fmt.Sprintf("/v/d%02d.md", i)})
	}
	got := s.list(1)
	if len(got) != 64 {
		t.Fatalf("%d kept, want 64", len(got))
	}
	if got[0] != "/v/d69.md" || got[63] != "/v/d06.md" {
		t.Fatalf("first %q last %q", got[0], got[63])
	}
	s.add(2, []string{"/v/x.md", "/v/x.md"})
	s.add(2, []string{"/v/x.md"})
	if got := s.list(2); len(got) != 1 {
		t.Fatalf("listed %v", got)
	}
}

func TestDocSeenSkipsMissingFile(t *testing.T) {
	dir := t.TempDir()
	a := writeDoc(t, dir, "A/plan.md", "a")
	b := writeDoc(t, dir, "B/plan.md", "b")
	s := &docSeen{}
	s.add(1, []string{a, b}) // b is newest
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	if got := s.find(1, "plan.md"); got != a {
		t.Fatalf("got %q, want %q", got, a)
	}
	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}
	if got := s.find(1, "plan.md"); got != "" {
		t.Fatalf("got %q, want none", got)
	}
}

func TestDocSeenOthersAndPrune(t *testing.T) {
	p := writeDoc(t, t.TempDir(), "tickets/V3-1/plan.md", "p")
	s := &docSeen{}
	s.add(2, []string{p})
	if got := s.find(1, "tickets/V3-1/plan.md"); got != "" {
		t.Fatalf("tab 1 found %q", got)
	}
	if got := s.findOthers(1, []tabs.TabID{1, 2}, "tickets/V3-1/plan.md"); got != p {
		t.Fatalf("others found %q, want %q", got, p)
	}
	s.prune([]*tabs.Tab{{ID: 1}})
	if got := s.findOthers(1, []tabs.TabID{1, 2}, "tickets/V3-1/plan.md"); got != "" {
		t.Fatalf("pruned tab still found %q", got)
	}
}

func TestDocSeenConcurrent(t *testing.T) {
	p := writeDoc(t, t.TempDir(), "a.md", "a")
	s := &docSeen{}
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 100 {
				s.add(tabs.TabID(g%2+1), []string{p, fmt.Sprintf("/v/%d-%d.md", g, i)})
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	for i := 0; ; i++ {
		s.find(1, "a.md")
		s.findOthers(1, []tabs.TabID{1, 2}, "a.md")
		select {
		case <-done:
			if got := s.find(1, "a.md"); got != p {
				t.Fatalf("got %q", got)
			}
			return
		default:
		}
		if i > 1_000_000 {
			t.Fatal("adds never finished")
		}
	}
}
