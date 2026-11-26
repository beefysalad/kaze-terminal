package tabs_test

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/ptyx"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestSpawnOptsPassArgvEnvAndMetadata(t *testing.T) {
	st := &starter{}
	var commands [][]string
	m := tabs.NewManagerWithArgv(func(argv, env []string, dir string) (ptyx.Session, error) {
		commands = append(commands, append([]string(nil), argv...))
		return st.start(env, dir)
	}, 12, 4, nil)
	t.Cleanup(m.CloseAll)
	m.SetNotify("127.0.0.1:1234", "test-token")
	parent := mustNew(t, m)
	dir := t.TempDir()
	argv := []string{"/test/agent", "--", "quotes '\" & %\n日本語"}
	task := "KAZE_TASK=quotes '\" & %\n日本語"
	worker, err := m.NewOpts(tabs.StartOpts{Argv: argv, Env: []string{task, "EXTRA_TEST=value"}, Dir: dir, Background: true, Parent: parent.ID, Role: "worker"})
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || commands[0] != nil || !reflect.DeepEqual(commands[1], argv) {
		t.Fatalf("commands = %q, want nil then %q", commands, argv)
	}
	if st.dirs[1] != dir {
		t.Fatalf("dir = %q, want %q", st.dirs[1], dir)
	}
	for _, want := range []string{task, "EXTRA_TEST=value", "KAZE_TAB_ID=2", "KAZE_NOTIFY=127.0.0.1:1234", "KAZE_NOTIFY_TOKEN=test-token", "KAZE_SOCKET=127.0.0.1:1234"} {
		name, _, _ := strings.Cut(want, "=")
		if n, got := countPrefix(st.envs[1], name+"="); n != 1 || got != want {
			t.Errorf("env %s: count %d, value %q, want %q", name, n, got, want)
		}
	}
	if got := m.Lookup(worker.ID); got.Parent != parent.ID || got.Role != "worker" {
		t.Fatalf("worker metadata = parent %d role %q", got.Parent, got.Role)
	}
	if got := m.Active(); got == nil || got.ID != parent.ID {
		t.Fatalf("background spawn changed active tab: %v", got)
	}
	plain := mustNew(t, m)
	if plain.Parent != 0 || plain.Role != "" {
		t.Fatalf("plain tab inherited metadata: %v", plain)
	}
	if n, _ := countPrefix(st.envs[2], "KAZE_TASK="); n != 0 {
		t.Fatal("task leaked to sibling tab")
	}
	if m.Active().ID != plain.ID {
		t.Fatal("ordinary tab did not become active")
	}
}

func TestBackgroundSpawnWithoutActiveTab(t *testing.T) {
	m, _ := newManager(t)
	t.Cleanup(m.CloseAll)
	tab, err := m.NewOpts(tabs.StartOpts{Background: true})
	if err != nil {
		t.Fatal(err)
	}
	if m.Active() != nil || !m.Has(tab.ID) {
		t.Fatal("background tab must exist without becoming active")
	}
}

func TestOrchestratorEnvironmentDoesNotLeakIntoOrdinaryTabs(t *testing.T) {
	for _, name := range []string{"KAZE_TASK", "KAZE_PARENT", "KAZE_ROLE", "KAZE_TAB_KEY"} {
		t.Setenv(name, "inherited")
		t.Setenv(strings.ToLower(name), "inherited-lowercase")
	}
	m, st := newManager(t)
	t.Cleanup(m.CloseAll)
	mustNew(t, m)
	for _, e := range st.envs[0] {
		name, _, _ := strings.Cut(e, "=")
		switch strings.ToUpper(name) {
		case "KAZE_TASK", "KAZE_PARENT", "KAZE_ROLE", "KAZE_TAB_KEY":
			t.Errorf("inherited orchestration environment leaked: %q", e)
		}
	}
}

func TestLegacyStarterRefusesExplicitArgv(t *testing.T) {
	m, st := newManager(t)
	t.Cleanup(m.CloseAll)
	if _, err := m.NewOpts(tabs.StartOpts{Argv: []string{"/test/agent"}}); err == nil {
		t.Fatal("legacy starter silently ignored argv")
	}
	if len(st.fakes) != 0 || !m.Empty() {
		t.Fatal("refused argv started a shell")
	}
}

func TestArgvStartFailurePreservesActiveTab(t *testing.T) {
	st := &starter{}
	m := tabs.NewManagerWithArgv(func(argv, env []string, dir string) (ptyx.Session, error) {
		if len(argv) != 0 {
			return nil, io.ErrUnexpectedEOF
		}
		return st.start(env, dir)
	}, 12, 4, nil)
	t.Cleanup(m.CloseAll)
	parent := mustNew(t, m)
	if _, err := m.NewOpts(tabs.StartOpts{Argv: []string{"/test/agent"}, Background: true}); err != io.ErrUnexpectedEOF {
		t.Fatalf("start error = %v", err)
	}
	if got := m.Active(); got == nil || got.ID != parent.ID || len(m.Tabs()) != 1 {
		t.Fatal("failed spawn changed tabs or focus")
	}
}
