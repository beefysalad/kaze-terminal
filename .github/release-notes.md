## What’s new in Kaze 0.23

- **Find the active project:** its repo header gets a soft accent tint and accent-colored name and arrow alongside the selected session. That context stays visible in Settings, documents, floating panes and folded groups.
- **Clearer running marks:** a still dot inside a closed ring replaces the frozen-looking spinner across session rows, subagents, the title breadcrumb and the agents pill. No animation or extra idle frames.
- **Keep Kaze open:** closing the final session, or exiting its final plain shell, leaves a New session action and your shortcut ready. Settings, Workspaces and worktree cleanup dialogs remain usable.
- **Shift+Enter for a newline:** continue a Claude Code prompt without sending it. Plain Enter still submits, and Alt/Option+Enter remains available.

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
