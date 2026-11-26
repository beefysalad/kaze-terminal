## What's changed

- An agent can start Claude Code or Codex workers in background tabs with kaze run, wait for them, read their reports and stop them.
- Workers nest under their parent in the same space, with a worker mark, a status bar count and Stop all workers.
- Off by default: enable Settings > Agents > Let agents start workers. The first time an agent asks to start workers, Kaze asks you to allow it.
- Templates live in the config folder. Limits keep each agent to 4 workers and Kaze to 8 in total.
- Workers close with their parent and never come back after a restart. A worker waiting on a trust or approval dialog is left for you to answer.
- Only Claude Code and Codex can be workers. On Windows, Claude Code needs the native claude.exe.

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
