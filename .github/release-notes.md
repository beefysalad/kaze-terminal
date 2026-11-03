## What’s new in Kaze 0.25

- **A tidier agent tray:** each subagent takes one line under its session. At most three show, waiting first; the rest sit behind “+N more”, and “N agents” folds the whole tray. Hover a line to see what the agent is doing.
- **Safe clicks:** clicking an agent line shows its session, so a stray click lands on your terminal. Open an agent’s panel with the › at the end of the line, or a double-click.
- **Find your project:** a thin accent rail marks the project you’re in, instead of a tinted box. Sidebar text stays readable in every theme.
- **Colour emoji:** ✅ ❌ 🔄 ⏳ and friends draw in full colour on Windows and macOS, like your other terminals. Each one is drawn once and reused.
- **Fixed: title symbols:** Claude’s status symbols no longer show as an empty box in the title bar or the sidebar.

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
