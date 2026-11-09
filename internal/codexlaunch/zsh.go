package codexlaunch

import (
	"os"
	"path/filepath"
	"strings"
)

const userZdotEnv = "KAZE_ZSH_USER_ZDOTDIR"
const zshDirName = "zsh"

func isZsh(shell, goos string) bool {
	return goos != "windows" && filepath.Base(shell) == "zsh"
}

// IsZsh reports whether shell reads zsh startup files on this platform.
func IsZsh(shell string) bool { return isZsh(shell, platformName()) }

// A login shell's startup files reorder PATH (path_helper, Homebrew, nvm), so
// the launcher has to be put back in front after they have all run. These files
// load the user's own at the top level: inside a function, a plain typeset in
// them would become local.
func zshLoad(name string) string {
	return `if [[ -z "$__kaze_had" && "$__kaze_user" == "$HOME" ]]; then
  unset ZDOTDIR
else
  ZDOTDIR="$__kaze_user"
fi
[[ -r "$__kaze_user/` + name + `" ]] && source "$__kaze_user/` + name + `"
__kaze_user="${ZDOTDIR:-$HOME}"
ZDOTDIR="$__kaze_private"
`
}

const zshStart = `typeset -g __kaze_private="$ZDOTDIR" __kaze_user="${` + userZdotEnv + `:-$HOME}" __kaze_had="${` + userZdotEnv + `:+1}"
unset ` + userZdotEnv + `
__kaze_finish() {
  [[ -n "$` + markerEnv + `" ]] && path=("$` + markerEnv + `" "${(@)path:#$` + markerEnv + `}")
  if [[ -z "$__kaze_had" && "$__kaze_user" == "$HOME" ]]; then
    unset ZDOTDIR
  else
    export ZDOTDIR="$__kaze_user"
  fi
  unset __kaze_private __kaze_user __kaze_had
  unfunction __kaze_finish
}
`

// /etc/zshrc on macOS derives HISTFILE from ZDOTDIR while it is still private.
const zshHistory = `[[ "$HISTFILE" == "$__kaze_private/.zsh_history" ]] && HISTFILE="$__kaze_user/.zsh_history"
`

func zshFiles() map[string]string {
	return map[string]string{
		".zshenv":   zshStart + zshLoad(".zshenv") + "[[ ! -o login && ! -o interactive ]] && __kaze_finish\n",
		".zprofile": zshLoad(".zprofile"),
		".zshrc":    zshHistory + zshLoad(".zshrc") + "[[ ! -o login ]] && __kaze_finish\n",
		".zlogin":   zshLoad(".zlogin") + "__kaze_finish\n",
	}
}

// PrepareZsh makes Env start zsh through the private startup files. The user's
// files are read, never written.
func (l *Launcher) PrepareZsh() error {
	dir := filepath.Join(l.dir, zshDirName)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	for name, body := range zshFiles() {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			return err
		}
	}
	l.zdot = dir
	return nil
}

func (l *Launcher) zshEnv(env, out []string) []string {
	if l.zdot == "" {
		return out
	}
	kept := out[:0]
	for _, e := range out {
		k, _, _ := strings.Cut(e, "=")
		if k != "ZDOTDIR" && k != userZdotEnv {
			kept = append(kept, e)
		}
	}
	if user := envValue(env, "ZDOTDIR"); user != "" && !shimDir(filepath.Dir(user)) {
		kept = append(kept, userZdotEnv+"="+user)
	}
	return append(kept, "ZDOTDIR="+l.zdot)
}
