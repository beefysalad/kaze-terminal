## What’s new in Kaze 0.26

- **A new agents board:** filter by All, Needs you, Running or Done, and search by type, task, session or repo (press /). Waiting agents sit in a pinned Needs you card; the rest are grouped under their session, with a bar showing what’s waiting, running and done. Finished agents fold away.
- **Preview without leaving:** pick an agent to see its steps and messages, or its formatted report, beside the list. Double-click it, press Enter or use Open in session to jump to it.
- **One click opens an agent:** clicking an agent line in the sidebar opens its panel again, and a second click closes it. “+N more” opens the board for that session.
- **A quieter highlight:** the project rail is gone. The project you’re in keeps its bold name and your session keeps its own bar.
- **Fixed: report summaries:** a finished agent shows the first sentence of its report, not its heading.

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
