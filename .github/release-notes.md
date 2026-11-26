## What's changed

- kaze ls marks your own tab with a self column. An agent name like codex resolves to a tab in your own space first.
- Sending to, reading from or pressing keys in a tab of another space prints a warning.
- Peer messaging and worker instructions share one rule: use a tab in your own space, never another project's tab unless you named it. If none is available, start your own worker when worker instructions are installed; otherwise tell the owner. Peer instructions now include the PowerShell form.
- Settings shows Update available with an Update button for outdated instruction blocks, and Edited with Reset for blocks you changed. A one-time note at startup points out outdated instructions. If you already switched instructions on, press Update once in Settings > Agents.
- Instruction blocks saved with Windows line endings no longer show as Modified.
- Workers are told to talk only to their parent.

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
