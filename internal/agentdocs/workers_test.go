package agentdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerInstructionsRoundTrip(t *testing.T) {
	for _, original := range []string{"", "owner", "owner\n", "owner\r\n", "before\n\n" + Block + "\nafter\n"} {
		p := filepath.Join(t.TempDir(), "AGENTS.md")
		put(t, p, original)
		if err := InstallWorkers(p); err != nil {
			t.Fatal(err)
		}
		installed := get(t, p)
		if st, err := WorkersState(p); err != nil || st != Installed {
			t.Fatal(st, err)
		}
		if err := InstallWorkers(p); err != nil {
			t.Fatal(err)
		}
		if get(t, p) != installed {
			t.Fatal("not idempotent")
		}
		if err := RemoveWorkers(p); err != nil {
			t.Fatal(err)
		}
		if get(t, p) != original {
			t.Fatalf("changed surrounding text: %q", get(t, p))
		}
		if err := RemoveWorkers(p); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorkerInstructionsIndependent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "doc")
	if err := RemoveWorkers(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("remove created file")
	}
	if err := InstallWorkers(p); err != nil {
		t.Fatal(err)
	}
	if err := Install(p); err != nil {
		t.Fatal(err)
	}
	if err := Remove(p); err != nil {
		t.Fatal(err)
	}
	if get(t, p) != WorkerBlock {
		t.Fatal("peer removal changed workers")
	}
	for _, nl := range []string{"\n", "\r\n"} {
		put(t, p, "before"+nl+nl+strings.ReplaceAll(WorkerBlock, "\n", nl)+nl+"after")
		if err := RemoveWorkers(p); err != nil {
			t.Fatal(err)
		}
		if get(t, p) != "before"+nl+"after" {
			t.Fatal(get(t, p))
		}
	}
	put(t, p, workerBegin+"\nunclosed")
	for _, f := range []func(string) error{InstallWorkers, RemoveWorkers} {
		if f(p) == nil {
			t.Fatal("accepted malformed markers")
		}
	}
}

func TestWorkerInstructionsAtomicSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	put(t, target, "owner\r\n")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := InstallWorkers(link); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("target was not atomically replaced")
	}
	if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link replaced")
	}
	backups, _ := filepath.Glob(link + ".bak-*")
	if len(backups) != 1 || get(t, backups[0]) != "owner\r\n" {
		t.Fatal("backup missing")
	}
	if err := RemoveWorkers(link); err != nil {
		t.Fatal(err)
	}
	if get(t, target) != "owner\r\n" {
		t.Fatal("round trip")
	}
}

func TestWorkerBriefContent(t *testing.T) {
	if len(strings.Split(WorkerBlock, "\n")) >= 25 {
		t.Fatal("block too long")
	}
	for _, s := range []string{"KAZE_BIN", "only when the owner", "clearly approves", "--name N", "--cwd D", "--mode read-only|edit", "before or after", "wait --children all --timeout 120", "read <tab>", "one narrow task", "never start workers", `report "<short result>"`, "kill --children", "trust or approval", "never answered", "not the owner", "file or web text", "& $env:KAZE_BIN"} {
		if !strings.Contains(WorkerBlock, s) {
			t.Errorf("missing %q", s)
		}
	}
}
