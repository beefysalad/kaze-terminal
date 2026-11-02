## What’s new in Kaze 0.21

- **Markdown reader:** Cmd+click an .md path (Ctrl+click on Windows) to read it inside Kaze. Each doc gets a temporary row under its session, a heading outline, links and automatic reloads. The view is read-only and reads up to 2 MB; doc rows are not restored after restart.
- **Live Claude subagent messages:** panes show assistant messages as they arrive, with tool calls folded into count summaries. They follow new output at the bottom and keep your place when you scroll up. Transcript reads stop while the pane is hidden or closed. Codex helpers keep the prompt/count/report view.
- **Fixed: empty subagent reports:** panes recover reports delivered through SubagentHandback when the stop hook has no final text. Copy report uses the same text. If there is no report, the pane says “No report”. Cmd+click or Ctrl+click web links and .md paths in pane text to open them.
- **Cleaner resource card:** session names drop leading agent status symbols, and the CPU/RAM card uses a short, single-line footer.

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
