package codexlaunch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const depthEnv = "KAZE_CODEX_PROXY_DEPTH"

func IsInvocation(argv0 string) bool {
	name := strings.ToLower(filepath.Base(argv0))
	return name == "codex" || name == "codex.exe"
}

// Dispatch runs before any Kaze CLI/window handling. A renamed launcher with
// missing tab identity fails visibly rather than opening a GUI or sharing hooks.
func Dispatch(argv, env []string, stderr io.Writer) (bool, int) {
	if len(argv) == 0 || !IsInvocation(argv[0]) {
		return false, 0
	}
	fail := func(code int, err error) (bool, int) { fmt.Fprintln(stderr, err); return true, code }
	self, err := os.Executable()
	if err != nil {
		return fail(126, err)
	}
	if want := envValue(env, probeEnv); want != "" {
		m, e := loadManifest(filepath.Dir(self))
		if e != nil || m.Token != want {
			return fail(126, fmt.Errorf("kaze: invalid Codex launcher probe"))
		}
		return true, 0
	}
	if err = validate(self, env); err != nil {
		return fail(126, err)
	}
	depth := 0
	if raw := envValue(env, depthEnv); raw != "" {
		depth, err = strconv.Atoi(raw)
		if err != nil || depth < 0 {
			return fail(126, fmt.Errorf("kaze: invalid Codex launcher recursion depth"))
		}
	}
	if depth >= 8 {
		return fail(126, fmt.Errorf("kaze: Codex launcher recursion limit reached; check shell wrappers"))
	}
	command, err := Resolve(env, self, platformName())
	if err != nil {
		return fail(127, err)
	}
	if command.Notice != "" {
		fmt.Fprintln(stderr, command.Notice)
	}
	childEnv := make([]string, 0, len(env)+1)
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		if !strings.EqualFold(k, depthEnv) {
			childEnv = append(childEnv, e)
		}
	}
	childEnv = append(childEnv, depthEnv+"="+strconv.Itoa(depth+1))
	args := append(append([]string{}, command.Prefix...), Arguments(argv[1:])...)
	code, err := execute(command.Path, args, childEnv)
	if err != nil {
		return fail(code, fmt.Errorf("kaze: cannot run Codex: %w", err))
	}
	return true, code
}
