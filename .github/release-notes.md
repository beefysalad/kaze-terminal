## What’s new in Kaze 0.24

- **Agent panel:** click a subagent to open its panel under the terminal instead of a floating card. Activity lists each step with the agent’s own messages between them; Report shows the finished report, formatted. Drag the edge to any height; each session remembers its panel.
- **Formatted reports:** bold, code, lists, headings and tables render in the panel. Copy report copies the original markdown, and Open in reader shows it in the doc reader.
- **Copy and paste on Windows:** Ctrl+C copies a selection and otherwise still interrupts; Ctrl+V pastes text and screenshots. Typing clears an old selection, so Ctrl+C always stops Claude after you type.
- **Fixed: workspace setup tabs:** a member whose commands finished reads Ready instead of stopped, and the workspace header no longer counts it as down.

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
