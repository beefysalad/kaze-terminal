# kaze

A small, fast terminal with tabs, written in Go with [Gio](https://gioui.org). It runs on Windows and macOS.

## Build

Windows (the `-H=windowsgui` flag stops a console window from opening next to kaze):

```
go build -ldflags "-H=windowsgui" -o kaze.exe ./cmd/kaze
```

macOS:

```
go build -o kaze ./cmd/kaze
```

### Smart App Control (Windows)

With Smart App Control on, Windows sometimes blocks a fresh unsigned build ("An Application
Control policy has blocked this file"). Rebuild so the binary gets a different hash, for example:

```
go build -ldflags "-H=windowsgui -s -w -X main.r=12345" -o kaze.exe ./cmd/kaze
```

Change the number and rebuild until the binary is allowed. The same thing can happen to test
binaries; re-run `go test` with `-ldflags "-X main.r=<random>"`.

## Config

kaze reads one TOML file: `%AppData%\kaze\config.toml` on Windows,
`~/Library/Application Support/kaze/config.toml` on macOS. Bad values fall back to defaults and
show as warnings in the first tab. Settings: `font_size`, `font_family`, `shell`,
`scrollback_lines`, `copy_on_select`, `[colors]` and `[keys]`.
