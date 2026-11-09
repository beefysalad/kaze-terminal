package codexlaunch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func zshLauncher(t *testing.T) *Launcher {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	l, err := New(self)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	if err := l.PrepareZsh(); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestIsZshRecognizesOnlyZsh(t *testing.T) {
	for shell, want := range map[string]bool{"/bin/zsh": true, "zsh": true, "/bin/bash": false, "/opt/zsh-tools/fish": false} {
		if got := isZsh(shell, "darwin"); got != want {
			t.Errorf("%s=%v", shell, got)
		}
	}
	if isZsh("zsh", "windows") {
		t.Error("zsh startup files are not used on Windows")
	}
}

func TestPrepareZshPointsShellAtPrivateFilesAndRemembersUsers(t *testing.T) {
	l := zshLauncher(t)
	env := l.Env([]string{"PATH=/usr/bin", "ZDOTDIR=/custom/zsh"})
	private := envValue(env, "ZDOTDIR")
	if !strings.HasPrefix(private, l.dir) || envValue(env, userZdotEnv) != "/custom/zsh" {
		t.Fatal(env)
	}
	for _, f := range []string{".zshenv", ".zprofile", ".zshrc", ".zlogin"} {
		if _, err := os.Stat(filepath.Join(private, f)); err != nil {
			t.Fatal(err)
		}
	}
	nested := l.Env(env)
	if envValue(nested, userZdotEnv) != "" {
		t.Fatal("private folder was remembered as the user's", nested)
	}
}

func TestEnvLeavesZdotdirAloneWithoutPrepareZsh(t *testing.T) {
	self, _ := os.Executable()
	l, err := New(self)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if got := envValue(l.Env([]string{"ZDOTDIR=/custom/zsh"}), "ZDOTDIR"); got != "/custom/zsh" {
		t.Fatal(got)
	}
}

func TestCloseRemovesZshFiles(t *testing.T) {
	l := zshLauncher(t)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(l.dir); !os.IsNotExist(err) {
		t.Fatal("launcher folder left behind", err)
	}
}

// runZsh starts a real zsh with fixture dotfiles; each one moves another folder
// ahead on PATH, as Homebrew and nvm do.
func runZsh(t *testing.T, l *Launcher, dotdir string, extra []string, flags ...string) (lines []string, log string) {
	t.Helper()
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	home := t.TempDir()
	if dotdir == "" {
		dotdir = home
	}
	for _, f := range []string{".zshenv", ".zprofile", ".zshrc", ".zlogin"} {
		body := "export PATH=/kaze-test-front:$PATH\nprint -r -- " + f + " >> \"$HOME/log\"\ntypeset KAZE_TEST_TOP=kept\n"
		if err := os.WriteFile(filepath.Join(dotdir, f), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	script := `print -r -- "${PATH%%:*}"; print -r -- "${ZDOTDIR-unset}"; print -r -- "$HISTFILE"; print -r -- "$KAZE_TEST_TOP"; print -r -- "${__kaze_private-gone}"`
	cmd := exec.CommandContext(ctx, zsh, append(flags, "-c", script)...)
	cmd.Env = l.Env(append([]string{"HOME=" + home, "PATH=/usr/bin:/bin", "KAZE_TAB_ID=1"}, extra...))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("zsh: %v %s", err, out)
	}
	b, _ := os.ReadFile(filepath.Join(home, "log"))
	all := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(all) < 5 {
		t.Fatalf("output=%q", out)
	}
	return all[len(all)-5:], strings.Join(strings.Fields(string(b)), " ")
}

func TestRealZshLoginKeepsShimFirstAndRunsUserFiles(t *testing.T) {
	l := zshLauncher(t)
	got, log := runZsh(t, l, "", nil, "-l", "-i")
	if got[0] != l.dir {
		t.Fatalf("first PATH entry=%q want %q", got[0], l.dir)
	}
	if got[1] != "unset" || strings.Contains(got[2], l.dir) || got[3] != "kept" || got[4] != "gone" {
		t.Fatalf("got=%q", got)
	}
	if log != ".zshenv .zprofile .zshrc .zlogin" {
		t.Fatalf("order=%q", log)
	}
}

func TestRealZshCustomZdotdirIsRestored(t *testing.T) {
	l := zshLauncher(t)
	custom := t.TempDir()
	got, _ := runZsh(t, l, custom, []string{"ZDOTDIR=" + custom}, "-l", "-i")
	if got[0] != l.dir || got[1] != custom {
		t.Fatalf("got=%q", got)
	}
}

func TestRealZshInteractiveNonLoginFinishes(t *testing.T) {
	l := zshLauncher(t)
	got, log := runZsh(t, l, "", nil, "-i")
	if got[0] != l.dir || got[1] != "unset" || log != ".zshenv .zshrc" {
		t.Fatalf("got=%q log=%q", got, log)
	}
}

func TestZshenvLoadsWithZdotdirUnsetWhenUserHadNone(t *testing.T) {
	body := zshFiles()[".zshenv"]
	unset, load := strings.Index(body, "unset ZDOTDIR\nelse\n  ZDOTDIR="), strings.Index(body, `source "$__kaze_user/.zshenv"`)
	if unset < 0 || load < 0 || unset > load {
		t.Fatalf("user .zshenv does not see ZDOTDIR unset:\n%s", body)
	}
}

func TestRealZshUserZshenvCanChooseZdotdir(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	l := zshLauncher(t)
	home := t.TempDir()
	custom := filepath.Join(home, ".config", "zsh")
	if err := os.MkdirAll(custom, 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(home, ".zshenv"), []byte(`export ZDOTDIR="${ZDOTDIR:-$HOME/.config/zsh}"`+"\n"), 0o600)
	os.WriteFile(filepath.Join(custom, ".zshrc"), []byte("export KAZE_TEST_RC=loaded\n"), 0o600)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, zsh, "-l", "-i", "-c", `print -r -- "$KAZE_TEST_RC|$ZDOTDIR|${PATH%%:*}"`)
	cmd.Env = l.Env([]string{"HOME=" + home, "PATH=/usr/bin:/bin", "KAZE_TAB_ID=1"})
	out, err := cmd.Output()
	if want := "loaded|" + custom + "|" + l.dir; err != nil || !strings.Contains(string(out), want) {
		t.Fatalf("err=%v out=%q want %q", err, out, want)
	}
}
