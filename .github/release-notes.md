## What’s new in Kaze 0.30

- **Find in the terminal:** press Ctrl+Shift+F on Windows or Cmd+F on a Mac to search the whole scrollback of Claude’s terminal or a server tab in the panel. Every match is highlighted and marked on the right edge. Enter and Shift+Enter step through them, Aa matches case, and Esc closes the bar and puts the view back where it was.
- **Built to stay out of the way:** new output updates the count without moving what you’re reading, a click in the terminal gives you the keyboard back, plain Ctrl+F still goes to the shell, and nothing runs while the bar is closed.

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
