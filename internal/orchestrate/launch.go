package orchestrate

import (
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

// WindowsCommandLineLimit leaves room below CreateProcess's 32,767-unit limit.
const WindowsCommandLineLimit = 30000

// LaunchOptions contains trusted host configuration. ClaudeExe is resolved on
// the tab's PATH with ResolveWindowsClaude. CodexLauncher is Kaze's private exe,
// not a user-supplied Codex wrapper. SessionID is a freshly generated UUID.
type LaunchOptions struct {
	GOOS, Shell, SessionID, ClaudeExe, CodexLauncher string
	Identity                                         Identity
}

// LaunchPlan is data only. Env is extra environment, not a replacement for the
// sanitized worker environment supplied by tabs. On Unix the shell unsets
// KAZE_TASK before exec; Windows never adds it.
type LaunchPlan struct{ Argv, Env []string }

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Plan constructs a launch without executing anything or resolving host paths.
func Plan(r Request, o LaunchOptions) (LaunchPlan, error) {
	message, err := BuildBrief(r, o.Identity)
	if err != nil {
		return LaunchPlan{}, err
	}
	r.Name = strings.TrimSpace(r.Name)
	flags := []string{}
	if r.Agent == "claude" {
		if !uuid.MatchString(o.SessionID) {
			return LaunchPlan{}, failure(CodeInvalidRequest, "Claude requires a session UUID")
		}
		flags = append(flags, "--session-id", o.SessionID)
		if r.Name != "" {
			flags = append(flags, "-n", r.Name)
		}
		if r.Mode == "read-only" {
			flags = append(flags, "--permission-mode", "plan")
		}
	} else {
		flags = append(flags, "--no-daemon")
		if r.Mode == "read-only" {
			flags = append(flags, "--sandbox", "read-only")
		}
	}
	if r.Model != "" {
		flags = append(flags, "--model", r.Model)
	}
	switch o.GOOS {
	case "darwin", "linux":
		base := path.Base(o.Shell)
		if !strings.HasPrefix(o.Shell, "/") || strings.ContainsRune(o.Shell, 0) || (base != "zsh" && base != "bash" && base != "sh") {
			return LaunchPlan{}, failure(CodeSpawnFailed, "worker shell must be an absolute zsh, bash or sh path")
		}
		script := `t="$KAZE_TASK"; unset KAZE_TASK; exec ` + r.Agent
		// Flags come from the table above. External values are always quoted.
		for i, flag := range flags {
			script += " "
			if i > 0 && (flags[i-1] == "--session-id" || flags[i-1] == "-n" || flags[i-1] == "--model") {
				script += quotePOSIX(flag)
			} else {
				script += flag
			}
		}
		script += ` -- "$t"`
		return LaunchPlan{Argv: []string{o.Shell, "-l", "-i", "-c", script}, Env: []string{"KAZE_TASK=" + message}}, nil
	case "windows":
		exe := o.ClaudeExe
		hint := "install the native claude.exe"
		if r.Agent == "codex" {
			exe = o.CodexLauncher
			hint = "private Codex launcher must be an absolute .exe path"
		}
		if !windowsAbsolute(exe) || !strings.HasSuffix(strings.ToLower(exe), ".exe") || strings.ContainsAny(exe, "\x00\r\n\"") || !utf8.ValidString(exe) {
			return LaunchPlan{}, failure(CodeSpawnFailed, hint)
		}
		argv := append([]string{exe}, flags...)
		argv = append(argv, "--", message)
		if WindowsCommandLineUnits(argv) >= WindowsCommandLineLimit {
			return LaunchPlan{}, failure(CodeTextTooLarge, "escaped Windows command line must be under 30000 UTF-16 units")
		}
		return LaunchPlan{Argv: argv}, nil
	default:
		return LaunchPlan{}, failure(CodeSpawnFailed, "unsupported worker operating system")
	}
}
func quotePOSIX(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
func windowsAbsolute(s string) bool {
	s = strings.ReplaceAll(s, "/", `\`)
	if len(s) >= 3 && ((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) && s[1] == ':' && s[2] == '\\' {
		return true
	}
	if strings.HasPrefix(s, `\\`) {
		parts := strings.Split(s[2:], `\`)
		return len(parts) >= 2 && parts[0] != "" && parts[1] != "" && parts[0] != "." && parts[0] != "?"
	}
	return false
}

// WindowsCommandLineUnits counts Go/ConPTY escaping, argument separators and
// the terminating NUL. Quotes and backslashes expand; non-BMP runes take 2 units.
// The first argument follows ComposeCommandLine's special executable quoting.
func WindowsCommandLineUnits(argv []string) int {
	units := 1
	for i, arg := range argv {
		if i > 0 {
			units++
		}
		if i == 0 {
			quoted := arg == "" || strings.HasPrefix(arg, `"`) || strings.IndexFunc(arg, func(r rune) bool { return r <= ' ' }) >= 0
			if quoted {
				units += 2
			}
			for _, r := range arg {
				if r != '"' || !quoted {
					units++
					if r > 0xffff {
						units++
					}
				}
			}
			continue
		}
		if arg == "" {
			units += 2
			continue
		}
		quoted := strings.ContainsAny(arg, " \t")
		if quoted {
			units += 2
		}
		slashes := 0
		for _, r := range arg {
			units++
			if r > 0xffff {
				units++
			}
			switch r {
			case '\\':
				slashes++
			case '"':
				units += slashes + 1
				slashes = 0
			default:
				slashes = 0
			}
		}
		if quoted {
			units += slashes
		}
	}
	return units
}

// ResolveWindowsClaude searches only the supplied tab environment and fs. It
// respects PATH/PATHEXT order and refuses the first wrapper, rather than silently
// launching a different installation later on PATH. No real filesystem is read.
func ResolveWindowsClaude(env []string, f FileSystem) (string, error) {
	if f == nil {
		return "", failure(CodeSpawnFailed, "filesystem must be supplied")
	}
	get := func(key string) string {
		for i := len(env) - 1; i >= 0; i-- {
			k, v, ok := strings.Cut(env[i], "=")
			if ok && strings.EqualFold(k, key) {
				return v
			}
		}
		return ""
	}
	extensions := get("PATHEXT")
	if extensions == "" {
		extensions = ".COM;.EXE;.BAT;.CMD"
	}
	exts := strings.Split(extensions, ";")
	dirs := strings.Split(get("PATH"), ";")
	if len(dirs) > 256 || len(exts) > 32 {
		return "", failure(CodeSpawnFailed, "worker executable search exceeds bounds")
	}
	for _, dir := range dirs {
		dir = strings.Trim(dir, `"`)
		if !windowsAbsolute(dir) {
			continue
		}
		for _, ext := range exts {
			// PATHEXT supplies extensions, never paths or arguments.
			if len(ext) < 2 || ext[0] != '.' || strings.ContainsAny(ext, `\/ :"`+"\x00\r\n\t") {
				continue
			}
			candidate := strings.TrimRight(strings.ReplaceAll(dir, "/", `\`), `\`) + `\claude` + strings.ToLower(ext)
			st, err := f.Stat(candidate)
			if err != nil || st == nil || !st.Mode().IsRegular() {
				continue
			}
			if !strings.EqualFold(ext, ".exe") {
				return "", failure(CodeSpawnFailed, "install the native claude.exe")
			}
			return candidate, nil
		}
	}
	return "", failure(CodeSpawnFailed, "cannot find claude.exe on the tab PATH; install the native claude.exe")
}
