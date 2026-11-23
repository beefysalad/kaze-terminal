## What's changed

- Spaces in the sidebar: one per project, plus a Shell space, with search and + in the header.
- Agents have their own section, grouped as Needs you, Working and Idle. Click an agent to jump to its tab.
- A tab strip above the terminal shows only the selected space's tabs.
- With the sidebar open, the title bar drops its + and search field. Open search with the magnifier in the sidebar header; Cmd/Ctrl+K stays unchanged.
- Settings opens as a floating sheet over the live terminal. Press Esc or click outside to close it.
- The Claude agent logo is now the Claude Code pixel mascot.
- The subagent tray under sidebar rows and line counts in sidebar rows are removed. Subagents remain in the dock panel and agents board.

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

Build from source instead: see the [README](https://github.com/beefysalad/kaze-terminal#build-and-run-from-a-clone).
