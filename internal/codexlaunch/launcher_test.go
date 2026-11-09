package codexlaunch

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMaterializeCopyFallbackAndWindowsOriginal(t *testing.T) {
	dir := t.TempDir()
	source := executableFixture(t, dir, "source", "source bytes")
	dest := filepath.Join(dir, "copied")
	if err := materialize(source, dest, "darwin", func(string, string) error { return errors.New("cross-device") }); err != nil {
		t.Fatal(err)
	}
	a, _ := os.Stat(source)
	b, _ := os.Stat(dest)
	if os.SameFile(a, b) {
		t.Fatal("fallback still shares source inode")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "source bytes" {
		t.Fatal(string(got))
	}
	pe := fakePE(0x20b)
	if err := os.WriteFile(source, pe, 0o700); err != nil {
		t.Fatal(err)
	}
	win := filepath.Join(dir, "codex.exe")
	if err := materialize(source, win, "windows", func(string, string) error { t.Fatal("Windows must not hardlink"); return nil }); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(source)
	if !bytes.Equal(original, pe) {
		t.Fatal("installed executable changed")
	}
	patched, _ := os.ReadFile(win)
	want, _ := ConsoleImage(pe)
	if !bytes.Equal(patched, want) {
		t.Fatal("Windows copy has wrong subsystem")
	}
}

func executableFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLauncherEnvironmentAndIndependentLifetime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixture; PE copy tested separately")
	}
	dir := t.TempDir()
	source := executableFixture(t, dir, "kaze", "#!/bin/sh\nexit 0\n")
	a, err := New(source)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := New(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if a.dir == b.dir {
		t.Fatal("launchers share directory")
	}
	base := []string{"PATH=" + dir, "CODEX_HOME=/custom", "KAZE_TAB_ID=4", "KAZE_NOTIFY=127.0.0.1:2", "KAZE_NOTIFY_TOKEN=t", depthEnv + "=8"}
	env := b.Env(a.Env(base))
	if envValue(env, "KAZE_TAB_ID") != "4" || envValue(env, "CODEX_HOME") != "/custom" || envValue(env, markerEnv) != b.dir {
		t.Fatal(env)
	}
	if envValue(env, depthEnv) != "0" {
		t.Fatal("nested app inherited proxy recursion depth", env)
	}
	path := envValue(env, "PATH")
	if strings.Contains(path, a.dir) || !strings.HasPrefix(path, b.dir+string(os.PathListSeparator)) {
		t.Fatal(path)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b.dir, shimName(runtime.GOOS))); err != nil {
		t.Fatal("closing first removed second", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("source removed")
	}
}

func TestResolveCurrentPathSkipsShimAndCopies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on Unix executable bits, which Windows files do not have")
	}
	dir := t.TempDir()
	own := filepath.Join(dir, "own")
	copyDir := filepath.Join(dir, "copy")
	realDir := filepath.Join(dir, "real")
	for _, p := range []string{own, copyDir, realDir} {
		if err := os.Mkdir(p, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	self := executableFixture(t, own, "codex", "same launcher")
	executableFixture(t, copyDir, "codex", "same launcher")
	real := executableFixture(t, realDir, "codex", "#!/bin/sh\necho real\n")
	env := []string{"PATH=" + strings.Join([]string{own, copyDir, realDir}, string(os.PathListSeparator))}
	cmd, err := Resolve(env, self, "darwin")
	if err != nil || cmd.Path != real {
		t.Fatalf("cmd=%+v err=%v", cmd, err)
	}
	if err := os.Chmod(real, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(env, self, "darwin"); err == nil {
		t.Fatal("accepted non-executable/recursive candidates")
	}
}

func TestResolveWindowsPATHEXTAndOfficialNPM(t *testing.T) {
	dir := t.TempDir()
	self := executableFixture(t, dir, "kaze.exe", "launcher")
	npm := filepath.Join(dir, "npm")
	os.Mkdir(npm, 0o700)
	executableFixture(t, npm, "codex.cmd", "@echo off\n")
	real := executableFixture(t, npm, "codex.exe", "native Codex")
	cmd, err := Resolve([]string{"Path=" + npm, "PATHEXT=.EXE;.CMD"}, self, "windows")
	if err != nil || cmd.Path != real {
		t.Fatalf("cmd=%+v %v", cmd, err)
	}
	if err := os.Remove(real); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(npm, "codex.cmd")
	if cmd, err := Resolve([]string{"PATH=" + npm, "PATHEXT=.CMD;.EXE"}, self, "windows"); err != nil || cmd.Path != wrapper || cmd.Notice == "" {
		t.Fatalf("unknown cmd wrapper=%+v %v", cmd, err)
	}
	pkg := filepath.Join(npm, "node_modules", "@openai", "codex")
	os.MkdirAll(filepath.Join(pkg, "bin"), 0o700)
	executableFixture(t, pkg, "package.json", `{"name":"@openai/codex","bin":{"codex":"bin/codex.js"}}`)
	js := executableFixture(t, filepath.Join(pkg, "bin"), "codex.js", "// official npm layout fixture")
	node := executableFixture(t, npm, "node.exe", "node")
	executableFixture(t, npm, "codex.cmd", npmCanonical)
	cmd, err = Resolve([]string{"PATH=" + npm, "PATHEXT=.CMD;.EXE"}, self, "windows")
	if err != nil || cmd.Path != node || len(cmd.Prefix) != 1 || cmd.Prefix[0] != js {
		t.Fatalf("npm command=%+v %v", cmd, err)
	}
	executableFixture(t, npm, "codex.cmd", npmCanonical+"echo custom\n")
	if cmd, err := Resolve([]string{"PATH=" + npm, "PATHEXT=.CMD;.EXE"}, self, "windows"); err != nil || cmd.Path != wrapper || cmd.Notice == "" {
		t.Fatalf("custom npm-adjacent wrapper=%+v %v", cmd, err)
	}
}

func TestLauncherDispatchRequiresMatchingMarkerAndTabIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixture")
	}
	source := executableFixture(t, t.TempDir(), "kaze", "#!/bin/sh\n")
	l, err := New(source)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	self := filepath.Join(l.dir, "codex")
	env := l.Env([]string{"KAZE_TAB_ID=2", "KAZE_NOTIFY=127.0.0.1:9", "KAZE_NOTIFY_TOKEN=token"})
	if err := validate(self, env); err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(l.dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(filepath.Join(canonical, "codex"), env); err != nil {
		t.Fatal("canonical temp-directory spelling rejected", err)
	}
	if err := validate(self, l.Env(nil)); err == nil {
		t.Fatal("accepted missing tab/notification identity")
	}
	if err := validate(source, env); err == nil {
		t.Fatal("accepted executable outside private directory")
	}
}
