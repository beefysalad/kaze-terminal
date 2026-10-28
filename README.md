# kaze

kaze is my personal terminal: a small, fast terminal with tabs, written in Go with
[Gio](https://gioui.org). I built it for working in [Claude Code](https://claude.com/claude-code)
all day, so it does a few things well (tabs, fast redraws, Japanese text, Claude Code's
symbols) and leaves out most of what a full terminal suite has. It has no settings screen;
everything lives in one small config file.

## Status

- **Windows:** works. This is what I use every day (Windows 11, PowerShell 7).
- **macOS:** untested. The code is written for macOS too, but the Mac milestone (running and
  benchmarking it on a real Mac) is still pending, so expect rough edges there.

## Requirements

- Go 1.27.0 or newer (see `go.mod`).
- macOS only: the Xcode Command Line Tools, because Gio uses cgo there:

  ```
  xcode-select --install
  ```

Windows needs nothing else; the Windows build uses no cgo.

## Install

With Go installed, this puts a `kaze` binary in your Go bin folder (`go env GOPATH`, then `bin`):

```
go install github.com/patrick-gogo/kaze-terminal/cmd/kaze@latest
```

On Windows, add the GUI flag so no console window opens next to kaze:

```
go install -ldflags "-H=windowsgui" github.com/patrick-gogo/kaze-terminal/cmd/kaze@latest
```

## Build and run from a clone

```
git clone https://github.com/patrick-gogo/kaze-terminal.git
cd kaze-terminal
```

Windows (`-H=windowsgui` stops a console window from opening next to kaze):

```
go build -ldflags "-H=windowsgui" -o kaze.exe ./cmd/kaze
.\kaze.exe
```

macOS:

```
go build -o kaze ./cmd/kaze
./kaze
```

The shell is PowerShell 7 (`pwsh`) on Windows, falling back to Windows PowerShell, and your
`$SHELL` (or `/bin/zsh`) as a login shell on macOS. Set `shell` in the config to use another;
if it fails to start, kaze falls back to the default shell and says so in the first tab. If no
shell can start at all, kaze shows the error in a message box on Windows (on macOS it prints
it to stderr) and exits.

### Smart App Control (Windows)

With Smart App Control on, Windows sometimes blocks a fresh unsigned build ("An Application
Control policy has blocked this file"). Rebuild so the binary gets a different hash, for example:

```
go build -ldflags "-H=windowsgui -s -w -X main.r=12345" -o kaze.exe ./cmd/kaze
```

Change the number and rebuild until the binary is allowed. The same thing can happen to test
binaries; re-run `go test` with `-ldflags "-X main.r=<random>"`.

## Shortcuts

| Action | Windows | macOS |
|---|---|---|
| New tab | Ctrl+Shift+T | Cmd+T |
| Close tab | Ctrl+Shift+W | Cmd+W |
| Next tab | Ctrl+Tab | Ctrl+Tab |
| Previous tab | Ctrl+Shift+Tab | Ctrl+Shift+Tab |
| Copy the selection | Ctrl+Shift+C | Cmd+C |
| Paste | Ctrl+Shift+V | Cmd+V |
| New line in Claude Code (without sending) | Alt+Enter | Option+Enter |

Plain Ctrl+C always goes to the shell as an interrupt. In Claude Code, Shift+Enter currently
submits like Enter, so use Alt+Enter (Option+Enter on macOS) for a new line. The tab, copy
and paste shortcuts can be changed under `[keys]` in the config.

## Mouse

- **Select:** drag with the left button. Copy with the copy shortcut, or turn on
  `copy_on_select` to copy when you let go. Dragging past the top or bottom edge scrolls.
- **Double-click** selects a word, **triple-click** a line. Keep dragging after the click to
  extend by words or lines.
- **Apps that use the mouse** (vim, htop and the like) get the clicks and the wheel. Hold
  **Shift** while dragging to select text in them anyway.
- **Wheel:** scrolls back through history. Typing jumps back to the bottom. Full-screen apps
  that don't use the mouse have no history to scroll.

## Config

kaze reads one TOML file at startup:

- Windows: `%AppData%\kaze\config.toml`
- macOS: `~/Library/Application Support/kaze/config.toml`

The file is optional. Restart kaze after editing it. A bad value falls back to its default and
shows up as a warning in the first tab; a TOML syntax error makes kaze use all defaults.

| Key | What it does | Default |
|---|---|---|
| `font_size` | Font size, 4 to 72 | `14` |
| `shell` | Program and arguments, e.g. `["pwsh", "-NoLogo"]` | PowerShell 7 / `$SHELL` |
| `scrollback_lines` | Lines of history per tab, up to 100000 | `10000` |
| `copy_on_select` | Copy a selection when the mouse button is released | `false` |
| `[colors]` | `fg`, `bg`, `cursor`, `selection`, `color0` to `color15`, as `"#rrggbb"` | the built-in Midnight palette |
| `[keys]` | `new_tab`, `close_tab`, `next_tab`, `prev_tab`, `copy`, `paste` | the shortcuts above |
| `font_family` | **Not supported yet.** Setting it only adds a warning; kaze always uses its bundled font. | |

Shortcuts are written like `"ctrl+shift+t"`. Modifiers: `ctrl`, `shift`, `alt` (or `option`),
`cmd`, `super` (or `win`). Keys: a letter or symbol, `tab`, `enter`, `space`, `esc`,
`backspace`, `delete`, `pageup`, `pagedown`, `home`, `end`, the arrows (`up`, `down`, `left`,
`right`) and `f1` to `f12`.

Example:

```toml
font_size = 15
shell = ["pwsh", "-NoLogo"]
scrollback_lines = 20000
copy_on_select = true

[colors]
bg = "#0f1117"

[keys]
new_tab = "ctrl+t"
```

kaze sets `TERM=xterm-256color`, `COLORTERM=truecolor` and `TERM_PROGRAM=kaze` for its shells,
plus `KAZE_TAB_ID` with the tab's number.

## Optional shell setup

PowerShell's own prediction and menu completion work well in kaze. To turn them on, add this
to your PowerShell profile (`notepad $PROFILE`):

```powershell
if ($Host.Name -eq 'ConsoleHost' -and -not [Console]::IsOutputRedirected) {
    Set-PSReadLineOption -PredictionSource History -PredictionViewStyle ListView
    Set-PSReadLineKeyHandler -Key Tab -Function MenuComplete
}
```

The guard keeps it quiet in scripts and other non-interactive sessions.

## Folder and branch in the status bar

kaze shows the active tab's folder and git branch (`~\src\kaze  ⎇ main`) on the left of the
status bar. The shell has to tell kaze where it is with an OSC 7 sequence from its prompt.
The branch is read from `.git/HEAD` whenever the prompt reports, so no git process runs.

PowerShell, in your profile:

```powershell
$__kazePrompt = $function:prompt
function prompt { $p = (Get-Location).ProviderPath -replace '\','/'; [Console]::Write("`e]7;file://$env:COMPUTERNAME/$p`a"); & $__kazePrompt }
```

zsh, in `~/.zshrc`:

```zsh
_kaze_osc7() { printf '\e]7;file://%s%s\a' "$HOST" "${PWD// /%20}" }
autoload -Uz add-zsh-hook; add-zsh-hook precmd _kaze_osc7
```

Without this the folder and branch stay hidden. UNC paths (`\server\share`) are not shown.

## Benchmarks

Numbers for the first milestone on Windows (key latency, Claude Code streaming, launch,
scrolling, idle CPU, memory), including the goals it missed:
[docs/benchmarks/2026-10-28-milestone-1.md](docs/benchmarks/2026-10-28-milestone-1.md).
`bench/` has the harness to run them yourself.

## Roadmap

Next:

- Alerts with a chime when a tab needs attention, kaze's own title bar, and the current folder
  and git branch in the status bar.
- Child tabs for Claude Code subagents.
- Claude Code usage in the status bar.
- The Mac milestone: run, fix and benchmark kaze on macOS.

Later:

- A browser button.
- A graph view of my notes vault.

## License

kaze's code is under the [MIT License](LICENSE). The bundled fonts in `assets/fonts`
(JetBrains Mono and a subset of Noto Sans JP) are not covered by it: each keeps its own
SIL Open Font License 1.1, in `assets/fonts/OFL-JetBrainsMono.txt` and
`assets/fonts/OFL-NotoSansJP.txt`, with sources in `assets/fonts/SOURCES.txt`.
