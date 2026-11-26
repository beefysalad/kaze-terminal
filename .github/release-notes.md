## What's changed

- After a send, the target counts as working until its next prompt starts, so wait no longer returns at once on the previous turn's idle state.
- Agents Kaze finds only by looking at the program in front of a tab, such as a Codex whose hooks did not reach Kaze, now show unknown instead of idle. Wait uses output quiet time for them.
- A request over the 64 KiB cap now fails with a clear message and exit code 2 instead of a silent end of input.
- A tag cannot be rebuilt out of nested pieces of a closing tag.

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
