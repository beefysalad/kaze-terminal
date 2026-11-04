## What’s new in Kaze 0.28

- **Read what your agents are doing:** a subagent’s panel and the agents board now read like Claude’s own transcript: the agent’s words first, then one sentence per step, like “Edited routes.ts +8 −5”, “Read 3 files” or “Ran the tests · failed · 1 test”. Click a step to see its diff or output. “Every step” brings back the full list.
- **Battery in the status bar:** on Windows laptops, the battery level sits at the far right of the status bar, with a bolt while charging, amber at 20% and red at 10%. It updates when Windows says it changed, so it costs nothing while idle. Desktops show nothing. macOS comes later.
- **Fixed: swapped agent names:** subagents of the same type started together keep their own task names instead of trading them.

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
