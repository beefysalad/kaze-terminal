//go:build windows

package codexlaunch

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWindowsConsoleProxyExactArgvNativeAndNPM(t *testing.T) {
	// A separately compiled helper avoids the resolver correctly identifying a
	// copied test binary as another private launcher. No real Codex is started.
	dir := t.TempDir()
	helper := filepath.Join(dir, "fakecodex.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	build := exec.CommandContext(ctx, "go", "build", "-o", helper, "./testdata/fakecodex")
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GO111MODULE=off", "GOWORK=off")
	if b, err := build.CombinedOutput(); err != nil {
		cancel()
		t.Fatalf("bounded fake helper build: %s %v", b, err)
	}
	cancel()
	binary, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"native", "npm"} {
		t.Run(kind, func(t *testing.T) {
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
			var prefix []string
			if kind == "native" {
				if err := os.WriteFile(filepath.Join(real, "codex.exe"), binary, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filepath.Join(real, "node.exe"), binary, 0o700); err != nil {
					t.Fatal(err)
				}
				executableFixture(t, real, "codex.cmd", npmCanonical)
				pkg := filepath.Join(real, "node_modules", "@openai", "codex")
				if err := os.MkdirAll(filepath.Join(pkg, "bin"), 0o700); err != nil {
					t.Fatal(err)
				}
				executableFixture(t, pkg, "package.json", `{"name":"@openai/codex","bin":{"codex":"bin/codex.js"}}`)
				js := executableFixture(t, filepath.Join(pkg, "bin"), "codex.js", "// fixture consumed by fake Node helper")
				prefix = []string{js}
			}
			args := []string{"resume", "", `quote"`, "日本語", "%VAR%", "!", "&", "|", "(x)", "^", "two words", `trailing\`}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, filepath.Join(l.dir, "CoDeX.ExE"), args...)
			cmd.Dir = cwd
			cmd.Env = l.Env([]string{"PATH=" + real, "PATHEXT=.EXE;.CMD", "KAZE_TAB_ID=10", "KAZE_NOTIFY=127.0.0.1:2", "KAZE_NOTIFY_TOKEN=token", "CODEX_HOME=C:\\custom", "SYSTEMROOT=" + os.Getenv("SYSTEMROOT")})
			cmd.Stdin = strings.NewReader("input text")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err = cmd.Run()
			if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 23 {
				t.Fatalf("exit=%v stderr=%s", err, stderr.String())
			}
			var got struct {
				Args                  []string
				Tab, Home, Cwd, Input string
			}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatal(err, string(stdout.Bytes()))
			}
			want := append(append([]string{}, prefix...), "--no-daemon")
			want = append(want, args...)
			if !reflect.DeepEqual(got.Args, want) || got.Tab != "10" || got.Home != "C:\\custom" || !strings.EqualFold(got.Cwd, cwd) || got.Input != "input text" || stderr.String() != "fake error" {
				t.Fatalf("got=%+v wantArgs=%q stderr=%s", got, want, stderr.String())
			}
		})
	}
}
