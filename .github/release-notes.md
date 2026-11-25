## What's changed

- An agent in one tab can read and type into another through the kaze command: ls, read, send, keys, interrupt, wait, name and label. Claude Code can hand work to Codex and back.
- A send arrives as one paste followed by Enter, tagged `<kaze from=...>` so the receiver can tell a peer from you.
- Wait blocks until the other agent finishes, without polling.
- Address a tab by id, by name, or by agent name when only one tab runs it.
- Settings, Agents has an off-by-default Peer messaging instructions switch for Claude Code and for Codex. It adds instructions to ~/.claude/CLAUDE.md or ~/.codex/AGENTS.md (or $CODEX_HOME/AGENTS.md) so every agent in every tab knows the commands. Turning it off removes exactly that block.
- Agent control uses the same local socket as the hooks and does no work when idle.
- A wrong kaze command now prints usage instead of opening a second window.

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
