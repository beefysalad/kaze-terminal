## What’s new in Kaze 0.27

- **Servers in the panel:** workspace members open as tabs in the panel under the terminal, next to your subagents, instead of floating over Claude. Click a member’s tab to type in it, click the terminal to take the keyboard back, and use Expand to make it the main view.
- **Richer alert cards:** a card at the bottom right names the session the way the sidebar does, shows its branch and slash command, Claude’s message or the start of its last reply, and how long the turn ran. Ctrl+Shift+J (⌘ J on macOS) opens the newest one.
- **Links with spaces:** a .md path with a space in it, or one printed after its folder, is underlined whole and opens on Ctrl+click. When it can’t be found, Kaze says where it looked.
- **Removed: the scratch shell and floating panes:** the docked panel replaces both. An old scratch_shell line in config.toml is ignored with a warning.

## Download

| | File |
|---|---|
| macOS 13 or newer (Apple Silicon and Intel) | `Kaze-macOS.zip` |
| Windows 10 or 11 (most PCs) | `Kaze-Windows-x64.zip` |
| Windows on ARM | `Kaze-Windows-arm64.zip` |

## Opening it the first time

Kaze isn't signed by Apple or Microsoft yet, so both warn you the first time.

**macOS:** unzip, then drag **Kaze** into Applications and open it. macOS says it can't verify the app: click **Done**, open **System Settings › Privacy & Security**, scroll down and click **Open Anyway** next to Kaze. You only do this once. (Or, in Terminal: `xattr -dr com.apple.quarantine /Applications/Kaze.app`.)

**Windows:** unzip and run `kaze.exe`. If SmartScreen says "Windows protected your PC", click **More info**, then **Run anyway**. If Smart App Control is on, it may block unsigned apps with no way past it; building from source works then.

Build from source instead: see the [README](https://github.com/patrick-gogo/kaze-terminal#build-and-run-from-a-clone).
