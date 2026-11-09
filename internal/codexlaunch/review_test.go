package codexlaunch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveWindowsEmptyPathEntryIsNotCurrentDirectory(t *testing.T) {
	cwd := t.TempDir()
	executableFixture(t, cwd, "codex.exe", "planted in a cloned repo")
	self := executableFixture(t, t.TempDir(), "kaze.exe", "launcher")
	t.Chdir(cwd)
	if cmd, err := Resolve([]string{"Path=;", "PATHEXT=.EXE"}, self, "windows"); err == nil {
		t.Fatalf("ran a program from the current folder: %+v", cmd)
	}
}

func TestLauncherWorksWithoutAlertListener(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	l, err := New(self)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	shim := filepath.Join(l.dir, shimName(runtime.GOOS))
	if err := validate(shim, l.Env([]string{"KAZE_TAB_ID=2"})); err != nil {
		t.Fatal("alert listener failure blocked Codex:", err)
	}
	if err := validate(shim, l.Env(nil)); err == nil {
		t.Fatal("accepted a launch with no owning tab")
	}
}

func TestNewDropsLauncherThatCannotStart(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var shim string
	_, err = newLauncher(self, func(path, _ string) error {
		shim = path
		return errors.New("blocked by policy")
	})
	if err == nil || !strings.Contains(err.Error(), "blocked by policy") {
		t.Fatalf("err=%v", err)
	}
	if shim == "" {
		t.Fatal("launcher was never tried")
	}
	if _, e := os.Stat(filepath.Dir(shim)); !os.IsNotExist(e) {
		t.Fatal("blocked launcher left on disk", e)
	}
}

func TestProbeStartsTheRealLauncher(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	l, err := New(self)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	shim := filepath.Join(l.dir, shimName(runtime.GOOS))
	if err := probeShim(shim, l.token); err != nil {
		t.Fatal(err)
	}
	if err := probeShim(shim, strings.Repeat("0", 32)); err == nil {
		t.Fatal("probe accepted the wrong token")
	}
}

func staleFixture(t *testing.T, root, name string, pid int, marker bool) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	executableFixture(t, dir, shimName(runtime.GOOS), "shim")
	if marker {
		source, _ := filepath.Abs("kaze")
		b, _ := json.Marshal(manifest{Version: 1, Token: strings.Repeat("a", 32), Source: source, PID: pid})
		if err := os.WriteFile(filepath.Join(dir, markerName), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSweepRemovesOnlyLaunchersOfDeadKazes(t *testing.T) {
	root := t.TempDir()
	dead := staleFixture(t, root, directoryPrefix+"dead", 999, true)
	live := staleFixture(t, root, directoryPrefix+"live", 111, true)
	unknown := staleFixture(t, root, directoryPrefix+"nopid", 0, true)
	noMarker := staleFixture(t, root, directoryPrefix+"nomarker", 999, false)
	other := staleFixture(t, root, "someone-elses", 999, true)
	sweep(root, func(pid int) bool { return pid == 111 })
	if _, e := os.Stat(dead); !os.IsNotExist(e) {
		t.Fatal("dead launcher kept", e)
	}
	for _, keep := range []string{live, unknown, noMarker, other} {
		if _, e := os.Stat(filepath.Join(keep, shimName(runtime.GOOS))); e != nil {
			t.Fatal("removed", keep, e)
		}
	}
}

func TestProcessAliveSeesThisProcess(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("own process reported dead")
	}
}
