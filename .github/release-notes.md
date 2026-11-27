## What's changed

- An agent can list and reach only tabs in its own space through the kaze command line. Other spaces are refused unless you set `[control] cross_space = true` in `config.toml`.
- A space is one repository. Workers share their parent's space, and tab names only have to be unique within a space.
- The Claude row icon now matches the real Claude Code mascot, with a wide body, arms, small eyes and four feet.
- Settings, Appearance, Sidebar has an Animated status switch (`animate_status`). Turn it on to spin and pulse the working mark while an agent works. It is off by default; leaving it off keeps idle CPU at zero.

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
