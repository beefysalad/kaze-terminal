# Kaze branding

The approved identity is the folded K with a terminal underscore. SVGs are real
paths and gradients, with outlined lettering and no fonts or embedded bitmaps.

- `kaze-mark.svg`: standalone violet mark on transparent background.
- `kaze-logo.svg` and `kaze-logo-light.svg`: README/full logo with attribution.
- `kaze-lockup.svg` and `kaze-lockup-light.svg`: compact logo without attribution.
  The app embeds the dark SVG, reads the `mark` and `word` groups once, and draws
  its own attribution and the matching purple gradient for its current theme.
- `kaze-icon.svg`: full tile for Windows and the website favicon.
- `kaze-app-icon-macos.svg`: icon artwork with macOS canvas margins.

The dark gradient runs from `#b9a6ff` to `#8b7bff` in mark coordinates
(0, 0) to (100, 112). The light contrast variant is `#7962d8` to `#6048bd`.
Keep these coordinates and colors in step with `internal/ui/lockup.go` and
`internal/ui/settings_about.go`. The three mark paths include the separate
underscore; keep the same geometry across all variants.

To regenerate the Windows PNG, all macOS ICNS sizes and website SVG copies:

```sh
npm ci --prefix tools/branding
npm run --prefix tools/branding export
```

Then regenerate the Windows executable resources from `cmd/kaze`:

```sh
go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out rsrc --arch amd64,arm64
```

`go test ./assets/branding ./internal/ui` checks platform exports and the actual
Gio gradient and terminal underscore. Do not change resource version metadata
for an artwork update; version bumps belong to the release workflow.
