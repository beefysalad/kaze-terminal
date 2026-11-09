package codexlaunch

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if IsInvocation(os.Args[0]) {
		_, code := Dispatch(os.Args, os.Environ(), os.Stderr)
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestProxyExactArgumentsEnvironmentStdioAndExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix exec fixture; Windows exact argv requires native runner")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	l, err := New(self)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	real := t.TempDir()
	cwd := t.TempDir()
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	executableFixture(t, real, "codex", "#!/bin/sh\nif [ \"$2\" = --help ]; then exit 0; fi\nprintf '%s\\000' \"$@\"\nprintf '%s\\000' \"$KAZE_TAB_ID\" \"$CODEX_HOME\" \"$PWD\"\nIFS= read -r line\nprintf '%s\\000' \"$line\"\nprintf 'fake error' >&2\nexit 23\n")
	args := []string{"resume", "", `quote"`, "日本語", "%VAR%", "!", "&", "|", "(x)", "^", "two words"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(l.dir, "codex"), args...)
	cmd.Dir = cwd
	cmd.Env = l.Env([]string{"PATH=" + real, "KAZE_TAB_ID=10", "KAZE_NOTIFY=127.0.0.1:2", "KAZE_NOTIFY_TOKEN=token", "CODEX_HOME=/custom"})
	cmd.Stdin = strings.NewReader("input text\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 23 {
		t.Fatalf("exit=%v stderr=%s", err, stderr.String())
	}
	got := strings.Split(strings.TrimSuffix(stdout.String(), "\x00"), "\x00")
	want := append([]string{"--no-daemon"}, args...)
	want = append(want, "10", "/custom", cwd, "input text")
	if !reflect.DeepEqual(got, want) || stderr.String() != "fake error" {
		t.Fatalf("stdout=%q want=%q stderr=%s", got, want, stderr.String())
	}
}

func TestProxyTwoTabsPreserveDistinctIdentityWithoutDaemon(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix fixture")
	}
	self, _ := os.Executable()
	l, err := New(self)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	real := t.TempDir()
	executableFixture(t, real, "codex", "#!/bin/sh\nif [ \"$1\" != '--no-daemon' ]; then printf 'stale daemon tab'; exit 1; fi\nprintf '%s' \"$KAZE_TAB_ID\"\n")
	for _, tab := range []string{"2", "10"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, filepath.Join(l.dir, "codex"))
		cmd.Env = l.Env([]string{"PATH=" + real, "KAZE_TAB_ID=" + tab, "KAZE_NOTIFY=127.0.0.1:2", "KAZE_NOTIFY_TOKEN=token"})
		b, err := cmd.CombinedOutput()
		cancel()
		if err != nil || string(b) != tab {
			t.Fatalf("tab %s got %s err %v", tab, b, err)
		}
	}
}

func TestDispatchOrdinaryAppAndInvalidPrivateLaunch(t *testing.T) {
	var out bytes.Buffer
	if yes, _ := Dispatch([]string{"kaze", "--version"}, nil, &out); yes {
		t.Fatal("ordinary app intercepted")
	}
	if yes, code := Dispatch([]string{"codex", "--version"}, nil, &out); !yes || code != 126 || !strings.Contains(out.String(), "launcher") {
		t.Fatalf("invalid dispatch %v %d %s", yes, code, out.String())
	}
}
