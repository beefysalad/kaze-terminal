// Package codexlaunch isolates local interactive Codex launches inside Kaze's
// own shell PATH. It never edits a user's CLI, profile or daemon configuration.
package codexlaunch

import "strings"

// Arguments preserves the original argv, adding the global isolation option
// only for recognized local interactive forms. Unknown CLI forms pass through.
func Arguments(args []string) []string {
	out := append([]string{}, args...)
	value := map[string]bool{"--config": true, "-c": true, "--enable": true, "--disable": true, "--image": true, "-i": true, "--model": true, "-m": true, "--local-provider": true, "--profile": true, "-p": true, "--sandbox": true, "-s": true, "--cd": true, "-C": true, "--add-dir": true, "--ask-for-approval": true, "-a": true}
	flag := map[string]bool{"--oss": true, "--approve-for-me": true, "--dangerously-bypass-approvals-and-sandbox": true, "--yolo": true, "--dangerously-bypass-hook-trust": true, "--worktree": true, "--search": true, "--no-alt-screen": true, "--strict-config": true, "--full-auto": true}
	interactive := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			interactive = true
			break
		}
		key, _, equals := strings.Cut(a, "=")
		if key == "--remote" || key == "--remote-auth-token-env" || key == "--no-daemon" || key == "--help" || key == "-h" || key == "--version" || key == "-V" {
			return out
		}
		if value[key] {
			if !equals {
				if i+1 == len(args) {
					return out
				}
				i++
			}
			continue
		}
		if flag[key] && !equals {
			continue
		}
		if interactive {
			if a == "--last" || a == "--all" {
				continue
			}
			if strings.HasPrefix(a, "-") {
				return out
			}
			continue
		}
		if a == "resume" || a == "fork" {
			interactive = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			return out
		}
		if strings.ContainsAny(a, " \t\r\n") {
			interactive = true
			continue
		}
		return out // single word could be an unknown future subcommand
	}
	return append([]string{"--no-daemon"}, out...)
}
